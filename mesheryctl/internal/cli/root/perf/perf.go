// Copyright 2019 The Meshery Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package perf

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/layer5io/meshery/mesheryctl/internal/cli/root/cfg"
	"github.com/pkg/errors"

	"github.com/layer5io/meshery/mesheryctl/pkg/utils"

	"github.com/layer5io/meshery/models"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/asaskevich/govalidator"
	"github.com/spf13/cobra"
)

var (
	testURL            = ""
	testName           = ""
	testMesh           = ""
	qps                = ""
	concurrentRequests = ""
	testDuration       = ""
	loadGenerator      = ""
	filePath           = ""
	tokenPath          = ""

	mctlCfg *cfg.MesheryCtl
)

// PerfCmd represents the Performance Management CLI command
var PerfCmd = &cobra.Command{
	Use:     "perf",
	Short:   "Performance Testing",
	Long:    `Performance Testing & Benchmarking using Meshery CLI.`,
	Example: "mesheryctl perf --name \"a quick stress test\" --url http://192.168.1.15/productpage --qps 300 --concurrent-requests 2 --duration 30s --token \"provider=Meshery\"",
	Args:    cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		//Check prerequisite
		return utils.PreReqCheck()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Importing SMPS Configuration from the file
		if filePath != "" {
			var t models.PerformanceSpec
			err := yaml.Unmarshal([]byte(filePath), &t)
			if err != nil {
				return errors.Wrapf(err, utils.PerfError(fmt.Sprintf("failed to unmarshal yaml file %s", filePath)))
			}
			if testDuration == "" {
				testDuration = fmt.Sprintf("%fs", t.EndTime.Sub(t.StartTime).Seconds())
			}
			if testURL == "" {
				testURL = t.EndpointURL
			}
			if concurrentRequests == "" {
				concurrentRequests = fmt.Sprintf("%d", t.Client.Connections)
			}
			if qps == "" {
				qps = fmt.Sprintf("%f", t.Client.Rps)
			}
		}

		if testName == "" {
			log.Debug("Test Name not provided")
			testName = utils.StringWithCharset(8)
			log.Debug("Using random test name: ", testName)
		}

		postData := ""

		startTime := time.Now()
		duration, err := time.ParseDuration(testDuration)
		if err != nil {
			return errors.Wrapf(err, utils.PerfError(fmt.Sprintf("failed to parse test duration %s", testDuration)))
		}

		endTime := startTime.Add(duration)

		postData = postData + "start_time: " + startTime.Format(time.RFC3339)
		postData = postData + "\nend_time: " + endTime.Format(time.RFC3339)

		if testURL != "" {
			postData = postData + "\nendpoint_url: " + testURL
		} else {
			return errors.New(utils.PerfError("please enter a test URL"))
		}

		// Method to check if the entered Test URL is valid or not
		var validURL = govalidator.IsURL(testURL)

		if !validURL {
			return errors.New(utils.PerfError("please enter a valid test URL"))
		}

		postData = postData + "\nclient:"
		postData = postData + "\n connections: " + concurrentRequests
		postData = postData + "\n rps: " + qps

		req, err := http.NewRequest("POST", mctlCfg.GetBaseMesheryURL(), bytes.NewBuffer([]byte(postData)))
		if err != nil {
			return errors.Wrapf(err, utils.PerfError(fmt.Sprintf("failed to create new request to %s", mctlCfg.GetBaseMesheryURL())))
		}

		if err := utils.AddAuthDetails(req, tokenPath); err != nil {
			return errors.Wrap(err, utils.PerfError("failed to add auth details to request"))
		}

		q := req.URL.Query()
		q.Add("name", testName)
		q.Add("loadGenerator", loadGenerator)
		if testMesh != "" {
			q.Add("mesh", testMesh)
		}
		req.URL.RawQuery = q.Encode()

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrapf(err, utils.PerfError(fmt.Sprintf("failed to make request to %s", testURL)))
		}
		log.Debug("Initiating Performance test ...")
		log.Debug(resp.Status)

		defer utils.SafeClose(resp.Body)
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, utils.PerfError("failed to read response body"))
		}
		log.Debug(string(data))

		if err := utils.UpdateAuthDetails(tokenPath); err != nil {
			return errors.Wrap(err, utils.PerfError("failed to update auth details"))
		}

		log.Debug("Test Completed Successfully!")
		return nil
	},
}

func init() {
	PerfCmd.Flags().StringVar(&testURL, "url", "", "(required) Endpoint URL to test")
	PerfCmd.Flags().StringVar(&testName, "name", "", "(optional) Name of the Test")
	PerfCmd.Flags().StringVar(&testMesh, "mesh", "", "(optional) Name of the Service Mesh")
	PerfCmd.Flags().StringVar(&qps, "qps", "0", "(optional) Queries per second")
	PerfCmd.Flags().StringVar(&concurrentRequests, "concurrent-requests", "1", "(optional) Number of Parallel Requests")
	PerfCmd.Flags().StringVar(&testDuration, "duration", "30s", "(optional) Length of test (e.g. 10s, 5m, 2h). For more, see https://golang.org/pkg/time/#ParseDuration")
	PerfCmd.Flags().StringVar(&tokenPath, "token", utils.AuthConfigFile, "(optional) Path to meshery auth config")
	PerfCmd.Flags().StringVar(&loadGenerator, "load-generator", "fortio", "(optional) Load-Generator to be used (fortio/wrk2)")
	PerfCmd.Flags().StringVar(&filePath, "file", "", "(optional) file containing SMPS-compatible test configuration. For more, see https://github.com/layer5io/service-mesh-performance-specification")
}
