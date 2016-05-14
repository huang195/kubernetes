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
// 2. If user defines a partial flavor, e.g., cpu=125m, we will fill out the rest of the flavor

// This plugin currently does not support using flavors by name, e.g., --requests="small" --limits="medium"
// This plugin should run after the Limitrange plugin so default values can be filled in before we check flavor,
// (make sure the default values are valid flavors)
// The ordering between limitrange and flavor plugins can be tricky if we want to use flavors by name. If flavor
// goes first, then it won't be able to check if default values are valid flavors (this is less of a problem if
// default value is configured to be the smallest flavor). If limitrange goes first, it won't be able to parse
// flavors by name and perform checks, e.g., (min <= req <= limits <= max), but this should also be fine since
// there's a validation step after all admission control plugins are finished.

// We are assuming the file specified by --admission-control-config-file=<file> is in key-value pairs
// In this file, we're going to scan for the key specified by FlavorConfigFile, and its value specifies
// the location of file that described the list of supported flavors.

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
			found = matchFlavor(v, reqs)
			break
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
		fv, exists := flavor[k]
		if !exists {
			return false
		}

		if fv != v {
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
