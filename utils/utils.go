package utils

import (
	"archive/zip"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-redis/redis/v7"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

type ReposInfo struct {
	Hash       uint32 `yaml:"hash"`
	Owner      string `yaml:"owner"`
	FolderName string `yaml:"folder_name"`
}

func (r *ReposInfo) Marshall() ([]byte, error) {
	return yaml.Marshal(r)
}

func (r *ReposInfo) Unmarshall(src string) error {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(content, r)
	return err
}

const (
	URL           = "http://localhost:5000/"
	ROOT          = "images"
	HiddenFile    = ".env.yml"
	MAX_FILE_SIZE = 8 << 20 // 8 MiB
	ROOT_PP       = "avatar/"
	MEDIA_ROOT    = "media"
	MEDIA_URL     = "media/"
)

func Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func GetRepoFile(name string) string {
	fileList := ""
	files, _ := ioutil.ReadDir(".")
	for _, file := range files {
		if strings.Split(file.Name(), ".")[1] != "zip" {
			fileList += file.Name()
		}
	}
	return fileList
}

func CreateFlatZip(w io.Writer, files ...string) error {
	z := zip.NewWriter(w)
	for _, file := range files {
		src, err := os.Open(file)
		if err != nil {
			return err
		}
		info, err := src.Stat()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = filepath.Base(file) // Write only the base name in the header
		dst, err := z.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
		src.Close()
	}
	return z.Close()
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewRedisDB(host, port, password string) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: password,
		DB:       0,
	})
	return redisClient
}
