package data

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
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

	target, err := ioutil.TempFile(path, "*.jpg")
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
		ch := Chat{}
		d.db.Find(&ch, id)
		if ch.ID == 0 {
			return "", errors.New("incorrect chat id")
		}

		ch.Avatar = url
		err = d.db.Save(&ch).Error
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

	size := src.Bounds().Max
	// do not resize small images
	if size.X > width || size.Y > height {
		dst := imaging.Thumbnail(src, width, height, imaging.Lanczos)
		err = imaging.Encode(target, dst, imaging.JPEG)
	} else {
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		bg := color.RGBA{255, 255, 255, 255} //  R, G, B, Alpha
		draw.Draw(dst, dst.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

		offset := image.Point{
			(width - size.X) / 2,
			(height - size.Y) / 2,
		}
		draw.Draw(dst, src.Bounds().Add(offset), src, image.Point{}, draw.Over)
		err = imaging.Encode(target, dst, imaging.JPEG)
	}

	if err != nil {
		return err
	}
	return nil
}
