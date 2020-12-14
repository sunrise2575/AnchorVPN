package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

type myDatabaseType struct {
	session *bolt.DB
}

var (
	db myDatabaseType
)

// Init : initializes database package
func (d *myDatabaseType) init(path string) error {
	var err error
	d.session, err = bolt.Open(path, 0600, nil)
	if err != nil {
		return err
	}

	err = d.session.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("vpn"))
		if err != nil {
			return fmt.Errorf("Create bucker: %s", err)
		}
		return nil
	})

	return err
}

// Quit : closed DB connection
func (d *myDatabaseType) quit() error {
	var err error
	if d.session != nil {
		err = d.session.Close()
	}

	return err
}

// CheckIP : Tells wether this IP exists or not
func (d *myDatabaseType) checkIP(i IP) (bool, error) {
	v, err := d.getIP(i)
	if (v == Profile{} && err == nil) {
		return false, nil
	} else if (v != Profile{} && err == nil) {
		return true, nil
	} else {
		return false, err
	}
}

// PutIP : Put one IP to databse
func (d *myDatabaseType) putIP(p Profile) error {
	encoded, err := json.Marshal(p)
	if err != nil {
		return err
	}

	err = d.session.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("vpn"))
		err := b.Put([]byte(p.IP), encoded)
		return err
	})

	return err
}

// GetIP : Get a profile (value) corresponding to an IP (key)
func (d *myDatabaseType) getIP(i IP) (Profile, error) {
	var isEmpty bool
	var v Profile

	err := d.session.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("vpn"))
		data := b.Get([]byte(i))

		if data == nil {
			isEmpty = true
			return nil
		}

		err := json.Unmarshal(data, &v)
		if err != nil {
			return err
		}

		return nil
	})

	if isEmpty {
		return Profile{}, nil
	}

	return v, err
}

// GetAll : Get all values in one bucket
func (d *myDatabaseType) getAll() ([]Profile, error) {
	result := []Profile{}
	err := d.session.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("vpn"))
		b.ForEach(func(k, v []byte) error {
			var tmp Profile
			e := json.Unmarshal(v, &tmp)
			if e != nil {
				return e
			}
			result = append(result, tmp)
			//fmt.Printf("key=%s, value=%s\n", k, v);
			return nil
		})
		return nil
	})

	return result, err
}

// DeleteIP : Delete a profile, matching with given IP address from DB
func (d *myDatabaseType) deleteIP(i IP) error {
	err := d.session.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("vpn"))
		err := b.Delete([]byte(i))
		return err
	})

	return err
}

// DeleteTime : aaa
func (d *myDatabaseType) deleteTime(serverInterface string, unusedTime time.Duration, leaseTime time.Duration) error {
	err := d.session.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("vpn"))
		b.ForEach(func(k, v []byte) error {
			var tmp Profile
			e := json.Unmarshal(v, &tmp)
			if e != nil {
				return e
			}

			// Check time
			tm := wgGetLatestHandshake(serverInterface, tmp.PublicKey)
			if tm.Sub(tmp.LastSeen) > 0 {
				tmp.LastSeen = tm
				vNew, e := json.Marshal(tmp)
				if e != nil {
					return e
				}
				b.Put(k, vNew)
			}

			flag := false
			// Delete time
			if (tmp.Created == tmp.LastSeen) && time.Now().Sub(tmp.LastSeen) > unusedTime {
				flag = true
				go lg.out("Delete (Issued but unused instantly): " + tmp.IP)
			} else if time.Now().Sub(tmp.LastSeen) > leaseTime {
				flag = true
				go lg.out("Delete (Lease time duration expired): " + tmp.IP)
			}

			if flag {
				e := b.Delete(k)
				if e != nil {
					lg.out("DB error: " + e.Error())
				}
				supportRun("wg set " + serverInterface + " peer " + tmp.PublicKey + " remove")
			}
			return nil
		})
		supportRun("wg-quick save " + serverInterface)
		return nil
	})
	return err
}
