package main

import (
	"flag"
	"fmt"
	"github.com/k0sf/gorm-sqlite3-n/wxSQLite3"
)

func main() {
	var name string
	var srcPass, newPass string
	flag.StringVar(&name, "name", "db.db", "-name=db.db")
	flag.StringVar(&srcPass, "src", "", "-src=[old password]")
	flag.StringVar(&newPass, "new", "", "-new=[new password]")
	flag.Parse()
	if name == "" {
		fmt.Println("No database (SQLite3) file specified！ ")
	}
	if srcPass == newPass {
		fmt.Println("No need to change password. ")
		return
	}

	err := wxSQLite3.ResetDBKey(name, srcPass, newPass)
	if err != nil {
		fmt.Println("Fail! ", err)
		return
	}
	fmt.Println("Successful. ")
}
