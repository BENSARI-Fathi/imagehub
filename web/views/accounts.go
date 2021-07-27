package views

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BENSARI-Fathi/imagehub/models"
	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/web/auth"
	"github.com/BENSARI-Fathi/imagehub/web/db"
	"github.com/BENSARI-Fathi/imagehub/web/form"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Account struct {
	rd auth.AuthInterface
	tk auth.TokenInterface
	mg *db.MongoClient
}

func NewAccount(rd auth.AuthInterface, tk auth.TokenInterface, mg *db.MongoClient) *Account {
	return &Account{
		rd: rd,
		tk: tk,
		mg: mg,
	}
}

func (acc *Account) Login(c *gin.Context) {
	user := &models.User{}
	userLoginForm := &form.UserLoginForm{}
	err := c.BindJSON(userLoginForm)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, "Invalid json provided.")
		return
	}
	// check if user exist
	filter := bson.D{{Key: "email", Value: userLoginForm.Email}}
	err = acc.mg.UserCollection.FindOne(context.Background(), filter).Decode(user)
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid email: %s", userLoginForm.Email))
		return
	}
	// verify password validity
	if !utils.CheckPasswordHash(userLoginForm.Password, user.Password) {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid Password."))
		return
	}
	// generate new token
	td, err := acc.tk.CreateToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, err.Error())
		return
	}
	saveErr := acc.rd.CreateAuth(user.ID.Hex(), td)
	if saveErr != nil {
		c.JSON(http.StatusUnprocessableEntity, saveErr.Error())
		return
	}
	tokens := map[string]string{
		"access_token":  td.AccessToken,
		"refresh_token": td.RefreshToken,
	}
	c.JSON(http.StatusOK, tokens)
}

func (acc *Account) Logout(c *gin.Context) {
	//If metadata is passed and the tokens valid, delete them from the redis store
	metadata, _ := acc.tk.ExtractTokenMetadata(c.Request)
	if metadata != nil {
		deleteErr := acc.rd.DeleteTokens(metadata)
		if deleteErr != nil {
			c.JSON(http.StatusBadRequest, deleteErr.Error())
			return
		}
	}
	c.JSON(http.StatusOK, "Successfully logged out")
}

func (acc *Account) UpdateProfilePicture(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}
	// Check file size
	if file.Size > utils.MAX_FILE_SIZE {
		c.String(400, "File size exceeds 8M")
		return

	}
	// Check file type
	buffer := make([]byte, 512)
	f, _ := file.Open()
	defer f.Close()
	f.Read(buffer)
	fileType := http.DetectContentType(buffer)
	if fileType != "image/png" && fileType != "image/jpeg" {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("invalid file type %s", fileType))
		return
	}
	token, _ := acc.tk.ExtractTokenMetadata(c.Request)
	userID := token.UserId
	// fetch the user object
	user := &models.User{}
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "error while parsing objectID")
		return
	}
	filter := bson.D{{Key: "_id", Value: oid}}
	err = acc.mg.UserCollection.FindOne(context.Background(), filter).Decode(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "error while parsing user object")
		return
	}

	mediaDir := utils.ROOT_PP + user.Username + "/"
	err = os.MkdirAll(mediaDir, 0755)
	if err != nil {
		c.JSON(400, err.Error())
		return
	}
	filename := filepath.Base(file.Filename)

	c.SaveUploadedFile(file, mediaDir+filename)
	imageUrl := fmt.Sprintf("http://%s/%s%s", c.Request.Host, mediaDir, filename)

	user.Avatar = imageUrl
	_, errUpdate := acc.mg.UserCollection.ReplaceOne(context.Background(), filter, user)
	if errUpdate != nil {
		c.JSON(http.StatusInternalServerError, "error while updating profile picuture.")
	}

	c.JSON(200, gin.H{
		"success": true,
		"url":     imageUrl,
	})

}

func (acc *Account) UserDetail(c *gin.Context) {
	_id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Error happen when fetching user ID.")
		return
	}
	filter := bson.M{"_id": _id}
	user := &models.User{}
	err = acc.mg.UserCollection.FindOne(context.Background(), filter).Decode(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Error happen when fetching user detail.")
		return
	}
	c.JSON(http.StatusOK, user)
}

func (acc *Account) Refresh(c *gin.Context) {
	mapToken := map[string]string{}
	if err := c.ShouldBindJSON(&mapToken); err != nil {
		c.JSON(http.StatusUnprocessableEntity, err.Error())
		return
	}
	refreshToken := mapToken["refresh_token"]

	//verify the token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_REFRESH_SECRET")), nil
	})
	//if there is an error, the token must have expired
	if err != nil {
		c.JSON(http.StatusUnauthorized, "Refresh token expired")
		return
	}
	//is token valid?
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		c.JSON(http.StatusUnauthorized, err)
		return
	}

	//Since token is valid, get the uuid:
	claims, ok := token.Claims.(jwt.MapClaims) //the token claims should conform to MapClaims
	if ok && token.Valid {
		refreshUuid, ok := claims["refresh_uuid"].(string) //convert the interface to string
		if !ok {
			c.JSON(http.StatusUnprocessableEntity, err.Error())
			return
		}
		// Check if the refreshUuid is valid
		_, err = acc.rd.FetchAuth(refreshUuid)
		if err != nil {
			c.JSON(http.StatusUnauthorized, "Invalid Refresh token")
			return
		}
		userId, roleOk := claims["user_id"].(string)
		if roleOk == false {
			c.JSON(http.StatusUnprocessableEntity, "unauthorized")
			return
		}
		//Delete the previous Refresh Token
		delErr := acc.rd.DeleteRefresh(refreshUuid)
		if delErr != nil { //if any goes wrong
			c.JSON(http.StatusUnauthorized, "unauthorized")
			return
		}
		//Create new pairs of refresh and access tokens
		ts, createErr := acc.tk.CreateToken(userId)
		if createErr != nil {
			c.JSON(http.StatusForbidden, createErr.Error())
			return
		}
		//save the tokens metadata to redis
		saveErr := acc.rd.CreateAuth(userId, ts)
		if saveErr != nil {
			c.JSON(http.StatusForbidden, saveErr.Error())
			return
		}
		tokens := map[string]string{
			"access_token":  ts.AccessToken,
			"refresh_token": ts.RefreshToken,
		}
		c.JSON(http.StatusCreated, tokens)
	} else {
		c.JSON(http.StatusUnauthorized, "refresh expired")
	}
}
