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
	"context"
	"fmt"
	"log"

	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/v1/pb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "check the lastest version of the repos",
	Long: `check the latest version of the repos in the db and 
compare it with the local repos`,
	Args:                  cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		check()
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// checkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// checkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func check() {
	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Error while trying to connect %v", err)
	}
	defer cc.Close()
	c := pb.NewImageReposClient(cc)
	reposInfo := &utils.ReposInfo{}
	err = reposInfo.Unmarshall(utils.HiddenFile)
	if err != nil {
		log.Fatal(err)
	}
	request := &pb.CheckRequest{
		Metadata: &pb.MetaData{
			Hash:       reposInfo.Hash,
			Owner:      reposInfo.Owner,
			FolderName: reposInfo.FolderName,
		},
	}
	resp, err := c.Check(context.Background(), request)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.GetStatus())
}
