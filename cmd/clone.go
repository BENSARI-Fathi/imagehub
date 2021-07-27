/*
Copyright © 2021 Fathi BENSARI <fethibensari@gmail.com>

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
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/v1/pb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var url string

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "clone a remote repository.",
	Long:  `clone a remote repository on your local machine.`,
	Run: func(cmd *cobra.Command, args []string) {
		clone(args)
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if url == "" && len(args) < 1 {
			return fmt.Errorf("accepts 1 arg(s)")
		}
		return nil
	},
	Example: `imagehub clone http://localhost:5000/<username>/<repos>
imagehub clone -u http://localhost:5000/<username>/<repos>`,
}

func init() {
	rootCmd.AddCommand(cloneCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	cloneCmd.PersistentFlags().StringVarP(&url, "url", "u", "", "remote repository url")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cloneCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func clone(args []string) {
	if url == "" {
		url = args[0]
	}
	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Error while trying to connect %v", err)
	}
	defer cc.Close()
	c := pb.NewImageReposClient(cc)
	req := &pb.CloneRequest{
		ReposPath: url,
	}
	respStream, err := c.Clone(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}
	// receive the hash value if the repository exist
	data, err := respStream.Recv()
	if err != nil {
		log.Fatalf("Error while reading stream %v\n", err)
	}
	//create directory
	directoryPath := filepath.Base(url)
	for {
		if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
			break
		}
		source := rand.NewSource(time.Now().UnixNano())
		r := rand.New(source)
		randomNumber := r.Int()
		directoryPath = fmt.Sprintf("%s-%v", directoryPath, randomNumber)
	}

	err = os.Mkdir(directoryPath, 0755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Chdir(directoryPath)
	if err != nil {
		log.Fatal(err)
	}

	// save the repository hash in local file
	metadata := &utils.ReposInfo{
		Hash:       data.GetMetadata().GetHash(),
		Owner:      data.GetMetadata().GetOwner(),
		FolderName: data.GetMetadata().GetFolderName(),
	}
	binaryData, err := metadata.Marshall()
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(utils.HiddenFile, binaryData, 0666)
	if err != nil {
		log.Fatal(err)
	}
	// setup the zipfile
	f, err := os.Create(directoryPath + ".zip")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	for {
		data, err := respStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error while reading stream %v\n", err)
		}
		f.Write(data.GetChunkData())
	}
	fmt.Println("[+] Cloned successfully")
}
