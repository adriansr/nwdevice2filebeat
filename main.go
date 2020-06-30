//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package main

import (
	"github.com/adriansr/nwdevice2filebeat/cmd"

	// Register outputs.
	_ "github.com/adriansr/nwdevice2filebeat/output/javascript"
	_ "github.com/adriansr/nwdevice2filebeat/output/logs"
	_ "github.com/adriansr/nwdevice2filebeat/output/logyml"
)

func main() {
	cmd.Execute()
}
