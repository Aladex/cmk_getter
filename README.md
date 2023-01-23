# cmk_getter

This project is a tool written in Go that downloads and manages the latest version of the deb package for Salt. This package is necessary for the correct operation of the Check_MK monitoring system on nodes located in the DMZ. The package contains all necessary dependencies and libraries that are required to run the monitoring system.

The tool has an built-in http server, which allows you to view the downloaded files, including their md5 sums. The utility automatically tracks the relevance of packages and applies the current version, replacing old packages.

Additionally, the project also includes an installer for Check_MK plugins for the free version. This feature allows you to install additional monitoring checks and data collection scripts on the nodes. This can be useful for monitoring the status of specific services or applications running on the nodes.

The project is open-source and is available on Github. Anyone can use, modify and distribute the code under the terms of the MIT License.

## Usage

The tool requires a config.yaml file in the same directory as the binary. The file should contain the following fields:

```yaml
listen: 0.0.0.0
port: 8080
domain: cmk.internal.domain.com
polling: 10
site: mysite
folders:
  - /path/to/folder1
  - /path/to/folder2
username: cmk_getter
password: generated_password
```

You can run the tool by executing the binary file:

```bash
./cmk_getter
```

The tool will listen on the IP address specified in the config file and the port specified in the config file. The domain, polling interval, site, and folders are also specified in the config file. The username and password fields are used for basic authentication when accessing the built-in web server.

You can change the IP address and port by modifying the config file. The polling interval is set in seconds, and determines how often the utility checks for new package versions.
