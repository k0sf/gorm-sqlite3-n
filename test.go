package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/k0sf/gorm-sqlite3-n/wxSQLite3"
	"gorm.io/gorm"
)

func main() {
	var db *gorm.DB
	var err error

	key := url.QueryEscape("my-pass")
	path := "./dbtest.db"
	path = "./newdata.db"
	// Path: "panel.db",
	//		Key:  "DHe$Fw3w5NAgMhFys$VLX35CvH3h",

	key = url.QueryEscape("DHe$Fw3w5NAgMhFys$VLX35CvH3h")
	path = "./panel.db"
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
		u := Users{Name: fmt.Sprintf("user%d", i)}
		//db.Table("users").Save(&u)
		db.Table("users").Create(&u)
	}

	db.Table("users").Limit(3).Find(&list)
	fmt.Println(list)
}
