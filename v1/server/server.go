package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BENSARI-Fathi/imagehub/v1/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/BENSARI-Fathi/imagehub/models"
	"github.com/BENSARI-Fathi/imagehub/utils"
)

var UserCollection, ImageCollection, RepositoryCollection *mongo.Collection

type Server struct {
	pb.UnimplementedImageReposServer
}

func (s *Server) Clone(req *pb.CloneRequest, stream pb.ImageRepos_CloneServer) error {
	fullPath := req.GetReposPath()
	// remove the url from path
	repos := strings.Split(fullPath, utils.URL)[1]
	// extract username
	path := strings.Split(repos, "/")
	username := path[0]
	folder := path[1]
	// verify if the user exists
	user := &models.User{}
	filter := bson.M{"username": username}
	res := UserCollection.FindOne(context.Background(), filter)
	if err := res.Decode(user); err != nil {
		return status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find user with the provided username: %s", username),
		)
	}
	// verify if the repos exists
	if _, err := os.Stat(fmt.Sprintf("%s/%s/%s", utils.ROOT, username, folder)); os.IsNotExist(err) {
		return status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find image repos with the provided folder name: %s", folder),
		)
	}
	// get the last version of the zipfile
	archive := &models.Archive{}
	opts := options.FindOne()
	opts.SetSort(bson.D{{Key: "_id", Value: -1}})
	filter2 := bson.D{
		{Key: "username", Value: username},
		{Key: "folder_name", Value: folder},
	}
	res = ImageCollection.FindOne(context.Background(), filter2, opts)
	if err := res.Decode(archive); err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot find the correct zipfile"),
		)
	}
	// send repos hash
	stream.Send(&pb.CloneResponse{
		Data: &pb.CloneResponse_Metadata{
			Metadata: &pb.MetaData{
				Hash:       archive.Hash,
				Owner:      archive.Username,
				FolderName: archive.FolderName,
			},
		},
	})
	//send data by chunk
	filename := fmt.Sprintf("./archive/%s/%s", username, archive.ZipFile)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println("le fichier est introuvable")
	}
	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("Internal Error"))
	}
	r := bufio.NewReader(f)
	buffer := make([]byte, 0, 4*1024)
	for {
		n, err := r.Read(buffer[:cap(buffer)])
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Error(codes.Internal, fmt.Sprintf("Internal Error here"))
		}
		buffer = buffer[:n]
		stream.Send(&pb.CloneResponse{
			Data: &pb.CloneResponse_ChunkData{
				ChunkData: buffer,
			},
		})
	}
	return nil
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {

	// check if user and username already exist
	username := req.GetUsername()
	email := req.GetEmail()
	filter := bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "username", Value: username}},
			bson.D{{Key: "email", Value: email}},
		}},
	}

	count, err := UserCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Internal Error"))
	}
	if count != 0 {
		return nil, status.Error(codes.Canceled, fmt.Sprintf("username: %s or email: %s already exists",
			username, email))
	}
	// check if password match password2
	if req.GetPassword() != req.GetPassword2() {
		return nil, status.Error(codes.Canceled, fmt.Sprintf("password don't match"))
	}
	// hash the password
	pwHash, _ := utils.HashPassword(req.GetPassword())

	// create user and save it to the db
	user := &models.User{
		Username: req.GetUsername(),
		Password: pwHash,
		Email:    req.GetEmail(),
	}
	resp, err := UserCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while creating %v", user),
		)
	}
	oid, ok := resp.InsertedID.(primitive.ObjectID)
	if !ok {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while parsing the inserted ID "),
		)
	}
	// create repos storage for the new user
	err = os.Mkdir(fmt.Sprintf("images/%s", username), 0755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(fmt.Sprintf("archive/%s", username), 0755)
	if err != nil {
		log.Fatal(err)
	}
	// send response to the client
	return &pb.RegisterResponse{
		Id:       oid.Hex(),
		Username: username,
		Email:    email,
	}, nil
}

func (s *Server) Push(stream pb.ImageRepos_PushServer) error {
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(
			codes.Unknown, fmt.Sprintf("Unknown error happen while receiving stream."),
		)
	}
	// verify user credential
	user := &models.User{}
	username := req.GetInfo().GetUsername()
	password := req.GetInfo().GetPassword()
	hash := req.GetInfo().GetHash()
	reposPath := req.GetInfo().GetReposPath()
	folderName := filepath.Base(reposPath)
	filter := bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "username", Value: username}},
			bson.D{{Key: "email", Value: username}},
		}},
	}
	err = UserCollection.FindOne(context.Background(), filter).Decode(&user)
	if !strings.Contains(reposPath, user.Username) {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("No repository found in %s", reposPath),
		})
	}
	if err != nil {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Invalid username field: %s", username),
		})
	}
	// verify password validity
	if !utils.CheckPasswordHash(password, user.Password) {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("The provided password is invalid"),
		})
	}
	// check if the repository exist otherwise create a new one
	filter = bson.D{

		{Key: "username", Value: username},
		{Key: "folder_name", Value: folderName},
	}
	count, _ := RepositoryCollection.CountDocuments(context.Background(), filter)
	if count == 0 {
		newRepos := &models.Repository{
			Username:   user.Username,
			FolderName: folderName,
			Timestamp:  primitive.Timestamp{T: uint32(time.Now().Unix())},
		}
		_, err = RepositoryCollection.InsertOne(context.Background(), newRepos)
		if err != nil {
			return stream.SendAndClose(&pb.PushResponse{
				Result: fmt.Sprintf("Internal Server Error while creating new repository"),
			})
		}
	}
	// create the zip file
	zipFileName := fmt.Sprintf("%s-%v.zip", folderName, hash)
	f, err := os.Create(zipFileName)
	defer f.Close()
	if err != nil {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Internal server error"),
		})
	}
	for {
		req, err = stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stream.SendAndClose(&pb.PushResponse{
				Result: fmt.Sprintf("Internal server error"),
			})
		}
		_, err = f.Write(req.GetChunkData())
		if err != nil {
			return stream.SendAndClose(&pb.PushResponse{
				Result: fmt.Sprintf("Internal server error"),
			})
		}
	}

	//unzip the file to images folder
	err = utils.Unzip(zipFileName, fmt.Sprintf("images/%s/%s", user.Username, folderName))
	if err != nil {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Internal server error"),
		})
	}

	// move the zip file into the archive
	err = os.Rename(zipFileName, fmt.Sprintf("archive/%s/%s", user.Username, zipFileName))
	if err != nil {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Internal server error"),
		})
	}

	// save the zipfile info in db if the zipfile doesn't exitst
	filter = bson.D{
		{Key: "username", Value: user.Username},
		{Key: "zip_file", Value: zipFileName},
	}
	count, err = ImageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Internal Server Error while checking the archives"),
		})
	}
	if count != 0 {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Successfully pushed to %s%s/%s", utils.URL, username, folderName),
		})
	}
	newArchive := &models.Archive{
		Username:   user.Username,
		Hash:       hash,
		ZipFile:    zipFileName,
		FolderName: folderName,
		Timestamp:  primitive.Timestamp{T: uint32(time.Now().Unix())},
	}
	resp, err := ImageCollection.InsertOne(context.Background(), newArchive)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while creating %v", user),
		)
	}
	_, ok := resp.InsertedID.(primitive.ObjectID)
	if !ok {
		return stream.SendAndClose(&pb.PushResponse{
			Result: fmt.Sprintf("Error while parsing the inserted ID "),
		})
	}
	return stream.SendAndClose(&pb.PushResponse{
		Result: fmt.Sprintf("Successfully pushed to %s%s/%s", utils.URL, username, folderName),
	})
}

func (s *Server) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	// get the metadata
	metadata := req.GetMetadata()
	// check if the provided version already exist
	filter := bson.D{
		{Key: "username", Value: metadata.GetOwner()},
		{Key: "folder_name", Value: metadata.GetFolderName()},
		{Key: "hash", Value: metadata.GetHash()},
	}
	count, err := ImageCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Internal Error"))
	}
	if count == 0 {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("The provided hash is invalid %v", metadata.GetHash()))
	}
	// check if the provided version is the last version
	archive := &models.Archive{}
	opts := options.FindOne()
	opts.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	filter2 := bson.D{
		{Key: "username", Value: metadata.GetOwner()},
		{Key: "folder_name", Value: metadata.GetFolderName()},
	}
	err = ImageCollection.FindOne(ctx, filter2, opts).Decode(archive)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Internal Error"))
	}
	result := archive.Hash == metadata.GetHash()
	// send an answer
	if result {
		return &pb.CheckResponse{
			Status: pb.CheckStatus_UpToDate,
		}, nil
	}
	return &pb.CheckResponse{
		Status: pb.CheckStatus_UpdateFound,
	}, nil
}

func main() {
	// connect to mongodb
	log.Println("Connecting to mongodb ....")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	UserCollection = client.Database("mydb").Collection("user")
	ImageCollection = client.Database("mydb").Collection("imagehub")
	RepositoryCollection = client.Database("mydb").Collection("repository")
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatalf("Disconnect error: %v", err)
		}
	}()
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("Can't listen %v", err)
	}
	s := grpc.NewServer()
	defer s.Stop()
	pb.RegisterImageReposServer(s, &Server{})
	log.Println("Grpc server listenning on 0.0.0.0:50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve %v\n", err)
	}
}
