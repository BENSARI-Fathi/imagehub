/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/v1/pb"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "push your local repository to a remote server",
	Long: `push your local repository to a remote server. For example:

imagehub push http://<servername>/<username>/<repositoryName>`,
	Run: func(cmd *cobra.Command, args []string) {
		push(args)
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pushCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pushCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func push(args []string) {

	var (
		username, fsum string
		fileList       []string
		localZip       = "imagehub.zip"
	)
	// setup grpc client
	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Error while trying to connect %v", err)
	}
	defer cc.Close()
	c := pb.NewImageReposClient(cc)

	remoteRepos := args[0]
	// calculate the hash
	files, err := ioutil.ReadDir("./")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		fsum += file.Name()
		fileList = append(fileList, file.Name())
	}
	hash := utils.Hash(fsum)
	// ask the client to provide credentials
	fmt.Print("Username: ")
	fmt.Scanln(&username)
	fmt.Print("Password: ")
	password, _ := gopass.GetPasswd()
	// create the zip
	for {
		if _, err := os.Stat(localZip); os.IsNotExist(err) {
			break
		}
		source := rand.NewSource(time.Now().UnixNano())
		r := rand.New(source)
		randomNumber := r.Int()
		fname := strings.Split(localZip, ".")
		localZip = fmt.Sprintf("%s-%v.%s", fname[0], randomNumber, fname[1])
	}
	f, err := os.Create(localZip)
	defer f.Close()
	err = utils.CreateFlatZip(f, fileList...)
	if err != nil {
		log.Fatal(err)
	}
	// reset the file cursor to the begining of the file
	f.Seek(0, 0)
	// send the metadata
	stream, err := c.Push(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	err = stream.Send(&pb.PushRequest{
		Data: &pb.PushRequest_Info{
			Info: &pb.UserCredentials{
				Username:  username,
				Password:  string(password),
				ReposPath: remoteRepos,
				Hash:      hash,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	// send the zip file chunk by chunk
	r := bufio.NewReader(f)
	buffer := make([]byte, 0, 4*1024)
	for {
		n, err := r.Read(buffer[:cap(buffer)])
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		buffer = buffer[:n]
		err = stream.Send(&pb.PushRequest{
			Data: &pb.PushRequest_ChunkData{
				ChunkData: buffer,
			},
		})
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.GetResult())
	os.Remove(localZip)
}
