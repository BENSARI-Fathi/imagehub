package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Username string             `bson:"username" json:"username"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"password"`
	Avatar   string             `bson:"avatar,omitempty" json:"avatar"`
}

type Archive struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	Username   string              `bson:"username" json:"username"`
	Hash       uint32              `bson:"hash" json:"hash"`
	ZipFile    string              `bson:"zip_file" json:"zip_file"`
	FolderName string              `bson:"folder_name" json:"folder_name"`
	Timestamp  primitive.Timestamp `bson:"timestamp" json:"timestamp"`
}

type Repository struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	Username   string              `bson:"username" json:"username"`
	FolderName string              `bson:"folder_name" json:"folder_name"`
	Timestamp  primitive.Timestamp `bson:"timestamp" json:"timestamp"`
}
