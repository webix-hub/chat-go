package data

import (
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func (d *ChatsDAO) UploadAvatar(file io.Reader, path string, server string) (string, error) {
	target, err := ioutil.TempFile(path, "*.jpg")
	if err != nil {
		return "", err
	}

	err = getImagePreview(file, 300, 300, target)
	if err != nil {
		return "", err
	}

	url := getAvatarURL(filepath.Base(target.Name()), server)
	return url, nil
}

func getAvatarURL(name, server string) string {
	return server + path.Join("/api/v1/chat", "0", "avatar", name)
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
