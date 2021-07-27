package views

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/BENSARI-Fathi/imagehub/models"
	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/web/auth"
	"github.com/BENSARI-Fathi/imagehub/web/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type imageFile struct {
	FileName string `json:"filename"`
	Url      string `json:"url"`
	// todo add url field
}

type repository struct {
	rd auth.AuthInterface
	tk auth.TokenInterface
	mg *db.MongoClient
}

func NewRepository(rd auth.AuthInterface, tk auth.TokenInterface, mg *db.MongoClient) *repository {
	return &repository{rd: rd, tk: tk, mg: mg}
}

func (rep *repository) GetRepos(c *gin.Context) {
	var repos []*models.Repository
	filter := bson.D{{}}
	opts := options.Find()
	opts.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := rep.mg.ReposCollecion.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	err = cursor.All(context.Background(), &repos)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, repos)
}

func (rep *repository) GetArchive(c *gin.Context) {
	var archives []*models.Archive
	filter := bson.D{{}}
	opts := options.Find()
	opts.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := rep.mg.ArchiveCollection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	err = cursor.All(context.Background(), &archives)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, archives)
}

func (rep *repository) GetFolderDetail(c *gin.Context) {
	_id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Error happen when fetching repository ID.")
		return
	}
	filter := bson.M{"_id": _id}
	repository := &models.Repository{}
	err = rep.mg.ReposCollecion.FindOne(context.Background(), filter).Decode(repository)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Error happen when fetching repository detail.")
		return
	}
	owner := repository.Username
	folder := repository.FolderName
	var images []imageFile

	files, err := ioutil.ReadDir(fmt.Sprintf("../%s/%s/%s", utils.ROOT, owner, folder))
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	for _, file := range files {
		f := imageFile{
			FileName: file.Name(),
			Url:      fmt.Sprintf("%s/%s%s/%s/%s", c.Request.Host, utils.MEDIA_URL, repository.Username, repository.FolderName, file.Name()),
		}
		images = append(images, f)
	}
	c.JSON(http.StatusOK, images)
}

func (rep *repository) GenerateReposUrl(c *gin.Context) {
	repos := &models.Repository{}
	var data map[string]string
	err := c.ShouldBind(&data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	_id, err := primitive.ObjectIDFromHex(data["_id"])
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	filter := bson.M{"_id": _id}
	err = rep.mg.ReposCollecion.FindOne(context.Background(), filter).Decode(repos)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	url := fmt.Sprintf("http://%s/%s/%s", c.Request.Host, repos.Username, repos.FolderName)
	c.JSON(http.StatusOK, gin.H{
		"url": url,
	})
}

func (rep *repository) SearchRepository(c *gin.Context) {
	var repos []*models.Repository
	query := c.Query("q")
	filter := bson.D{{Key: "folder_name", Value: query}}
	cursor, err := rep.mg.ReposCollecion.Find(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	err = cursor.All(context.Background(), &repos)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, repos)
}
