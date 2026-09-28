package models

import "time"

type VPNGroup struct {
	Id          int64
	Name        string    `orm:"size(64);unique"`
	Description string    `orm:"size(255);null"`
	Enabled     bool      `orm:"default(true)"`
	Created     time.Time `orm:"auto_now_add;type(datetime)"`
	Updated     time.Time `orm:"auto_now;type(datetime)"`
}

type NetworkResource struct {
	Id        int64
	Name      string    `orm:"size(64)"`
	CIDR      string    `orm:"size(64);unique"`
	Source    string    `orm:"size(16);default(manual)"`
	Interface string    `orm:"size(64);null"`
	Gateway   string    `orm:"size(64);null"`
	Enabled   bool      `orm:"default(false)"`
	Reachable bool      `orm:"default(true)"`
	LastSeen  time.Time `orm:"type(datetime);null"`
	Created   time.Time `orm:"auto_now_add;type(datetime)"`
	Updated   time.Time `orm:"auto_now;type(datetime)"`
}

type VPNUser struct {
	Id            int64
	Name          string    `orm:"size(64);unique"`
	CertificateCN string    `orm:"size(64);unique"`
	StaticIP      string    `orm:"size(64);unique"`
	Group         *VPNGroup `orm:"rel(fk);on_delete(do_nothing)"`
	Enabled       bool      `orm:"default(true)"`
	Created       time.Time `orm:"auto_now_add;type(datetime)"`
	Updated       time.Time `orm:"auto_now;type(datetime)"`
}

type GroupNetwork struct {
	Id      int64
	Group   *VPNGroup        `orm:"rel(fk);on_delete(cascade)"`
	Network *NetworkResource `orm:"rel(fk);on_delete(cascade)"`
	Created time.Time        `orm:"auto_now_add;type(datetime)"`
}
