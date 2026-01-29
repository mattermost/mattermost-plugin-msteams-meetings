// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

// These build-time vars are read from shell commands and populated in ../setup.mk
var (
	BuildHashShort  string
	BuildTagLatest  string
	BuildTagCurrent string
)

func main() {
	if len(os.Args) <= 1 {
		panic("no cmd specified")
	}

	manifest, err := findManifest()
	if err != nil {
		panic("failed to find manifest: " + err.Error())
	}

	cmd := os.Args[1]
	switch cmd {
	case "id":
		dumpPluginID(manifest)

	case "version":
		dumpPluginVersion(manifest)

	case "has_server":
		if manifest.HasServer() {
			fmt.Printf("true")
		}

	case "has_webapp":
		if manifest.HasWebapp() {
			fmt.Printf("true")
		}

	default:
		panic("unrecognized command: " + cmd)
	}
}

func findManifest() (*model.Manifest, error) {
	_, manifestFilePath, err := model.FindManifest(".")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find manifest in current working directory")
	}
	manifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", manifestFilePath)
	}
	defer manifestFile.Close()

	// Re-decode the manifest, disallowing unknown fields. When we write the manifest back out,
	// we don't want to accidentally clobber anything we won't preserve.
	var manifest model.Manifest
	decoder := json.NewDecoder(manifestFile)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return nil, errors.Wrap(err, "failed to parse manifest")
	}

	// If no version is listed in the manifest, generate one based on the state of the current
	// commit, and use the first version we find (to prevent causing errors)
	if manifest.Version == "" {
		var version string
		tags := strings.Fields(BuildTagCurrent)
		for _, t := range tags {
			if strings.HasPrefix(t, "v") {
				version = t
				break
			}
		}
		if version == "" {
			if BuildTagLatest != "" {
				version = BuildTagLatest + "+" + BuildHashShort
			} else {
				version = "v0.0.0+" + BuildHashShort
			}
		}
		manifest.Version = strings.TrimPrefix(version, "v")
	}

	// If no release notes specified, generate one from the latest tag, if present.
	if manifest.ReleaseNotesURL == "" && BuildTagLatest != "" {
		manifest.ReleaseNotesURL, err = url.JoinPath(manifest.HomepageURL, "releases", "tag", BuildTagLatest)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate release notes URL")
		}
	}

	return &manifest, nil
}

// dumpPluginId writes the plugin id from the given manifest to standard out
func dumpPluginID(manifest *model.Manifest) {
	fmt.Printf("%s", manifest.Id)
}

// dumpPluginVersion writes the plugin version from the given manifest to standard out
func dumpPluginVersion(manifest *model.Manifest) {
	fmt.Printf("%s", manifest.Version)
}
