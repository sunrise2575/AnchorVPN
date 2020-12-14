package main

import (
	"time"
)

// IP : IP address of a client, used as a key
type IP = string

// Profile : Profile information of a client, used as a value
type Profile struct {
	IP        IP
	PublicKey string
	//PrivateKey string
	Created  time.Time
	LastSeen time.Time
}
