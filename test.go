package main

import (
	"fmt"
	"github.com/k0sf/gorm-sqlite3-n/wxSQLite3"
	"gorm.io/gorm"
	"log"
	"net/url"
)

func main() {
	var db *gorm.DB
	var err error

	key := url.QueryEscape("pass")
	path := "./dbtest.db"
	//path = "./5"
	dsn := fmt.Sprintf("%s?_db_key=%s", path, key)

	db, err = gorm.Open(wxSQLite3.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	type Users struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	db.AutoMigrate(&Users{})
	var list []Users
	for i := 0; i < 5; i++ {
		u := Users{Name: fmt.Sprintf("%d", i)}
		db.Table("users").Save(&u)
	}

	db.Table("users").Limit(3).Find(&list)
	fmt.Println(list)
}
