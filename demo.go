package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"mkozhukh/chat/data"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
)

var demoDataFolder = "demodata"

func importDemoData(db *gorm.DB) {
	var c int
	db.Model(&data.User{}).Count(&c)

	if c > 0 {
		return
	}

	stat, err := os.Stat(demoDataFolder)
	if err != nil || !stat.IsDir() {
		return
	}

	importDemoStruct(db, data.User{})
	importDemoStruct(db, data.Message{})
	importDemoStruct(db, data.Chat{})
	importDemoStruct(db, data.UserChat{})
}

func importDemoStruct(db *gorm.DB, t interface{}) {
	dt := reflect.TypeOf(t)
	name := strings.ToLower(dt.Name())
	cont, err := ioutil.ReadFile(filepath.Join(demoDataFolder, name+".json"))
	if err != nil {
		return
	}

	log.Println("[demo-data]", name)

	slicePtr := reflect.New(reflect.SliceOf(dt))
	slice := slicePtr.Interface()

	err = json.Unmarshal(cont, &slice)
	if err != nil {
		log.Fatal(err)
	}

	sliceObj := slicePtr.Elem()
	for i := 0; i < sliceObj.Len(); i++ {
		el := sliceObj.Index(i)
		err = db.Save(el.Interface()).Error
		if err != nil {
			log.Fatal(err)
		}
	}
}
