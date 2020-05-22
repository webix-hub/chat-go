package data

import (
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"

	"github.com/disintegration/imaging"
)

func (d *ChatsDAO) UpdateAvatar(idStr string, file io.Reader, path string, server string) (string, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return "", err
	}

	target, err := ioutil.TempFile(filepath.Join(path), "*.jpg")
	if err != nil {
		return "", err
	}

	err = getImagePreview(file, 300, 300, target)
	if err != nil {
		return "", err
	}

	url := getAvatarURL(idStr, filepath.Base(target.Name()), server)
	// get existing chat
	if id != 0 {
		err = d.db.Table("chats").Where("chat_id = ?", id).Update("avatar", url).Error
		if err != nil {
			return "", err
		}
	}

	return url, nil
}

func getAvatarURL(id, name, server string) string {
	return server + path.Join("/api/v1/chat", id, "avatar", name)
}

func getImagePreview(source io.Reader, width, height int, target io.Writer) error {
	src, err := imaging.Decode(source)
	if err != nil {
		return err
	}

	dst := imaging.Thumbnail(src, width, height, imaging.Lanczos)
	err = imaging.Encode(target, dst, imaging.JPEG)

	if err != nil {
		return err
	}
	return nil
}
