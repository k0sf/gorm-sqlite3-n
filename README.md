# gorm-sqlite3-n

SQLCipher、wxSQLite3


## Install

```shell
go get -u github.com/k0sf/gorm-sqlite3-n@main
```


## wxSQLite3 ([to](https://github.com/Jathon-yang/go-wxsqlite3))

ps: 默认为AES128加密

```go

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
    dsn := fmt.Sprintf("%s?_db_key=%s", path, key)
    
    db, err = gorm.Open(wxSQLite3.Open(dsn), &gorm.Config{})
    if err != nil {
    log.Fatal(err)
    }
    
    type Users struct {
    Id     int64  `json:"id"`
    Name string `json:"name"`
    }
    var list []Users
    
    db.Table("users").Find(&list)
    fmt.Println(list)
}

```

### 设置或修改密码


```go
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

```

## SQLCipher ([to](https://github.com/jackfr0st13/gorm-sqlite-cipher))

```go

package main

import (
	"fmt"
	"github.com/k0sf/gorm-sqlite3-n/SQLCipher"
	"gorm.io/gorm"
	"log"
	"net/url"
)

func main() {
    var db *gorm.DB
    var err error

	key := url.QueryEscape("pass")
	path := "./dbtest.db"
    dsn := fmt.Sprintf("%s?_pragma_key=%s", path, key)
    
    db, err = gorm.Open(SQLCipher.Open(dsn), &gorm.Config{})
    if err != nil {
    log.Fatal(err)
    }
    
    type Users struct {
    Id     int64  `json:"id"`
    Name string `json:"name"`
    }
    var list []Users
    
    db.Table("users").Find(&list)
    fmt.Println(list)
}

```