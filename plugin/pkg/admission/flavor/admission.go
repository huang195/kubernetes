package flavor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// This plugin perform the following tasks
// 1. Ensures all containers are of defined flavors, and reject if they're not
// 2. If user defines a partial flavor, e.g., memory=64M, we will fill out the rest of the flavor

// This plugin should run after the Limitrange plugin so default values can be filled in before we check flavor,
// (make sure the default values are valid flavors)

// We are assuming the file specified by --admission-control-config-file=<file> is in key-value pairs.
// In this file, we're going to scan for the key specified by FlavorConfigFile, and its value specifies
// the location of file that described the list of supported flavors.

// This is an example configuration file with a list of supported flavors
//{
//	"flavors": {
//		"pico": {
//			"memory": "64Mi",
//			"cpu": "100m"
//		},
//		"nano": {
//			"memory": "128Mi",
//			"cpu": "125m"
//		},
//		"micro": {
//			"memory": "256Mi",
//			"cpu": "150m"
//		},
//		"tiny": {
//			"memory": "512Mi",
//			"cpu": "200m"
//		},
//		"small": {
//			"memory": "1024Mi",
//			"cpu": "250m"
//		},
//		"medium": {
//			"memory": "2048Mi",
//			"cpu": "500m"
//		},
//		"large": {
//			"memory": "4096Mi",
//			"cpu": "1"
//		},
//		"xlarge": {
//			"memory": "8192Mi",
//			"cpu": "2"
//		},
//		"2xlarge": {
//			"memory": "16384Mi",
//			"cpu": "4"
//		}
//	}
//}

const (
	FlavorConfigFile = "flavor.config"
)

func init() {
	admission.RegisterPlugin("Flavor", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewFlavor(config), nil
	})
}

type flavorType string

type flavors struct {
	Flavors map[flavorType]api.ResourceList
}

type flavor struct {
	*admission.Handler
	flavors flavors
}

func (f *flavor) Admit(a admission.Attributes) error {

	// ignore non pods
	if a.GetKind().GroupKind() != api.Kind("Pod") {
		return nil
	}

	// ignore calls with subresources defined
	if a.GetSubresource() != "" {
		return nil
	}

	// ignore calls with requested resources not pods
	if a.GetResource().Resource != "pods" {
		return nil
	}

	// check and fill requests against supported flavors
	var found bool
	pod := a.GetObject().(*api.Pod)
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		reqs := container.Resources.Requests

		// if reqs is not specified, we will reject it
		if len(reqs) == 0 {
			break
		}

		for _, v := range f.flavors.Flavors {
			if found = matchFlavor(v, reqs); found {
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("Request does not match any of the supported flavors")
	}
	return nil
}

func matchFlavor(flavor api.ResourceList, req api.ResourceList) bool {
	resources := make(map[api.ResourceName]bool)
	for k, _ := range flavor {
		resources[k] = true
	}

	for k, v := range req {
		glog.Infof("k = %v, v = %v\n", k, v)
		fv, exists := flavor[k]
		if !exists {
			glog.Infof("%v does not exist\n", k)
			return false
		}
		if fv.Cmp(v) != 0 {
			glog.Infof("%v != %v\n", fv, v)
			return false
		}
		delete(resources, k)
	}

	for k, _ := range resources {
		req[k] = flavor[k]
	}
	return true
}

func NewFlavor(config io.Reader) admission.Interface {
	if config == nil {
		glog.Fatalf("Flavor admission plugin requires `--admission-control-config-file` to be specified")
	}

	// Scans config to get the location of the file describing all the supported flavors
	var flavorConfigFile string
	scanner := bufio.NewScanner(config)
	for scanner.Scan() {
		b := scanner.Bytes()
		s := strings.Split(string(b), "=")
		if len(s) == 2 {
			key, value := s[0], s[1]
			if key == FlavorConfigFile {
				flavorConfigFile = value
				break
			}
		}
	}

	if flavorConfigFile == "" {
		glog.Fatalf("Flavor admission plugin requires flavor config file to be specified in `--admission-control-config-file`")
	}

	// Read the flavor config file and de-marshal
	b, err := ioutil.ReadFile(flavorConfigFile)
	if err != nil {
		glog.Fatalf("Cannot read flavor config file '%s': %v", flavorConfigFile, err)
	}

	var flavors flavors
	err = json.NewDecoder(bytes.NewReader(b)).Decode(&flavors)
	if err != nil {
		glog.Fatalf("Cannot decode flavor config file '%s': %v", flavorConfigFile, err)
	}

	return &flavor{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		flavors: flavors,
	}
}
