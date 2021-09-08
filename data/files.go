package data

import (
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type FilesDAO struct {
	dao *DAO
	db  *gorm.DB
}

type File struct {
	ID     int    `gorm:"primary_key"`
	Name   string `gorm:"column:name"`
	Path   string `gorm:"column:path"`
	UID    string `gorm:"column:uid"`
	ChatID int    `gorm:"chat_id"`
}

func NewFilesDAO(dao *DAO, db *gorm.DB) FilesDAO {
	return FilesDAO{dao: dao, db: db}
}

func (d *FilesDAO) PostFile(id int, file io.ReadSeeker, name, path, server string) error {
	target, err := ioutil.TempFile(path, "*")
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, file)
	if err != nil {
		return err
	}
	file.Seek(0, 0)

	tf := File{Name: name, Path: target.Name(), ChatID: id, UID: uuid.New().String()}
	err = d.db.Save(&tf).Error
	if err != nil {
		return err
	}

	url := getFileURL(server, tf.UID, name)
	mText := url + "\n" + name

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".jpg" || ext == ".png" || ext == ".gif" {

		previewName := target.Name() + ".preview"
		preview, err := os.Create(previewName)
		if err != nil {
			return err
		}
		defer preview.Close()

		err = getImagePreview(file, 300, 300, preview)
		if err != nil {
			return err
		}

		mText = mText + "\n" + getPreviewURL(server, tf.UID, name)
	}

	msg := Message{
		Text:    mText,
		Date:    time.Now(),
		ChatID:  id,
		UserID:  0,
		Type:    AttachedFile,
		Related: tf.ID,
	}
	return d.dao.Messages.SaveAndSend(id, &msg)
}

func (d *FilesDAO) GetOne(id string) *File {
	f := File{}
	d.db.First(&f, "uid = ?", id)
	return &f
}

func getFileURL(server, uid, name string) string {
	return server + path.Join("/api/v1/files", uid, name)
}

func getPreviewURL(server, uid, name string) string {
	return server + path.Join("/api/v1/files", uid, "preview", name)
}
