package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"
)

type UserMap struct {
	usersFile string
	m         unsafe.Pointer
}

func NewUserMap(usersFile string, done <-chan bool, onUpdate func()) *UserMap {
	um := &UserMap{usersFile: usersFile}
	m := make(map[string]bool)
	atomic.StorePointer(&um.m, unsafe.Pointer(&m))
	if usersFile != "" {
		log.Printf("using authenticated emails file %s", usersFile)
		started := WatchForUpdates(usersFile, done, func() {
			um.LoadAuthenticatedEmailsFile()
			onUpdate()
		})
		if started {
			log.Printf("watching %s for updates", usersFile)
		}
		um.LoadAuthenticatedEmailsFile()
	}
	return um
}

func (um *UserMap) IsValid(email string) (result bool) {
	m := *(*map[string]bool)(atomic.LoadPointer(&um.m))
	_, result = m[email]
	return
}

func (um *UserMap) LoadAuthenticatedEmailsFile() {
	r, err := os.Open(um.usersFile)
	if err != nil {
		log.Fatalf("failed opening authenticated-emails-file=%q, %s", um.usersFile, err)
	}
	defer r.Close()
	csv_reader := csv.NewReader(r)
	csv_reader.Comma = ','
	csv_reader.Comment = '#'
	csv_reader.TrimLeadingSpace = true
	records, err := csv_reader.ReadAll()
	if err != nil {
		log.Printf("error reading authenticated-emails-file=%q, %s", um.usersFile, err)
		return
	}
	updated := make(map[string]bool)
	for _, r := range records {
		updated[strings.ToLower(r[0])] = true
	}
	atomic.StorePointer(&um.m, unsafe.Pointer(&updated))
}

func newValidatorImpl(domains []string, usersFile string,
	done <-chan bool, onUpdate func()) func(string) bool {
	validUsers := NewUserMap(usersFile, done, onUpdate)

	for i, domain := range domains {
		domains[i] = fmt.Sprintf("@%s", strings.ToLower(domain))
	}

	validator := func(email string) bool {
		email = strings.ToLower(email)
		valid := false
		for _, domain := range domains {
			valid = valid || strings.HasSuffix(email, domain)
		}
		if !valid {
			valid = validUsers.IsValid(email)
		}
		log.Printf("validating: is %s valid? %v", email, valid)
		return valid
	}
	return validator
}

func NewValidator(domains []string, usersFile string) func(string) bool {
	return newValidatorImpl(domains, usersFile, nil, func() {})
}
