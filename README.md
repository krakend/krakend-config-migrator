# KrakenD version migration
This application migrates existing KrakenD configurations from one major version to the other (e.g: 1.x to 2.0).

The application **overwrites** the target files with the resulting new configuration. It's avised that you keep your configuration files under a git tree to review or revert changes easily.

The application works with any file extension of your choice (patterns) but it defaults to `*.json` (simple configuration) or `*.tmpl` (usually flexible configuration).

You can migrate KrakenD configurations with thousands of files/templates in seconds.

Make sure to review the changes after the migration.

## Usage
Download the binary for your platform or build the source code (see below). From the folder where the binay is, execute it:

	$ ./krakend-updater -h
	Usage of ./krakend-updater:
	  -c int
	    	concurrency level (default 24)
	  -m string
	    	path to a custom mapping definition
	  -p string
	    	patterns to use to contain the file modification (default "*.json,*.tmpl")

All flags are optional and usually not needed, unless you want to use custom rules, you use YAML configuration, or other non-standard options. 

Pass all your configuration folders as arguments. E.g: if you want to migrate 3 different KrakenD projects at once:

	$ ./krakend-updater /path/to/project1 /path/to/project2 /path/to/project3

The passed folder will contain any needed modification to use the latest KrakenD configuration version.

## Build
If you need to make modifications to this migration tool:

1- Clone the repo

	git clone git@gitlab.com:devops-faith/migrate-krakend-version.git

2- Modify it and build it:

	cd migrate-krakend-version
	go build ./cmd/krakend-updater
