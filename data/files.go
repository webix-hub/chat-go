package data

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
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

func (d *FilesDAO) PostFile(id, uid int, file io.ReadSeeker, name, path, server string) error {
	target, err := ioutil.TempFile(path, "*")
	if err != nil {
		return err
	}
	defer target.Close()

	tf, size, err := d.copyFile(id, name, path, file, target)
	if err != nil {
		return err
	}

	url := getFileURL(server, tf.UID, name)
	sizeStr := strconv.Itoa(int(size))
	mText := url + "\n" + name + "\n" + sizeStr

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".jpeg" || ext == ".jpg" || ext == ".png" || ext == ".gif" {
		err = createPreview(file, mText, target.Name())
		if err != nil {
			log.Println("can't create preview:" + err.Error())
		} else {
			mText = mText + "\n" + getPreviewURL(server, tf.UID, name)
		}
	}

	msg := Message{
		Text:    mText,
		Date:    time.Now(),
		ChatID:  id,
		UserID:  uid,
		Type:    AttachedFile,
		Related: tf.ID,
	}
	return d.dao.Messages.SaveAndSend(id, &msg, "", 0)
}

func (d *FilesDAO) PostVoice(id, uid int, file io.ReadSeeker, duration, name, path, server string) error {
	target, err := ioutil.TempFile(path, "*")
	if err != nil {
		return err
	}
	defer target.Close()

	tf, _, err := d.copyFile(id, name, path, file, target)
	if err != nil {
		return err
	}

	url := getFileURL(server, tf.UID, name)
	mText := url + "\n" + duration

	msg := Message{
		Text:    mText,
		Date:    time.Now(),
		ChatID:  id,
		UserID:  uid,
		Type:    VoiceMessage,
		Related: tf.ID,
	}

	return d.dao.Messages.SaveAndSend(id, &msg, "", 0)
}

func (d *FilesDAO) copyFile(id int, name, path string, file io.ReadSeeker, target *os.File) (File, int64, error) {
	size, err := io.Copy(target, file)
	if err != nil {
		return File{}, 0, err
	}
	file.Seek(0, 0)

	tf := File{Name: name, Path: target.Name(), ChatID: id, UID: uuid.New().String()}
	err = d.db.Save(&tf).Error

	return tf, size, err
}

func createPreview(file io.ReadSeeker, mText, name string) error {
	previewName := name + ".preview"
	preview, err := os.Create(previewName)
	if err != nil {
		return err
	}
	defer preview.Close()

	err = getImagePreview(file, 300, 300, preview)
	if err != nil {
		return err
	}

	return nil
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
