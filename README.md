# ByteSeeker

ByteSeeker is a performance-centered web scanner tool written in Go, designed to efficiently scan and identify accessible web paths on a given website. With concurrent scanning capabilities, adaptive request delaying, and customizable user agent strings, ByteSeeker offers a robust platform to discover potential vulnerabilities or enumerate resources on web servers.

## Features

- **Concurrent Scanning**: Utilizes goroutines for fast, concurrent web path scanning.
- **Adaptive Delay**: Dynamically adjusts the delay between requests to manage server load and evade rate limiting.
- **Custom User Agents**: Supports multiple user agents for scanning, mimicking different devices and browsers.
- **DNS Caching**: Implements DNS caching to reduce DNS lookup times and improve overall scanning efficiency.
- **Configurable Timeout**: Allows setting a custom timeout for HTTP requests to balance between speed and reliability.

## Prerequisites

Before you start using ByteSeeker, ensure you have the following installed:
- Go (version 1.14 or later)

## Installation

Clone this repository to your local machine and build the project:
- git clone https://github.com/hdunl/ByteSeeker.git
- cd ByteSeeker/cmd
- go build

## Usage

To use ByteSeeker, you need to specify the base URL you wish to scan and the file containing the paths to scan. Additional flags can be used to customize the scan.
- ./ByteSeeker -url http://example.com -paths paths.txt -concurrent 10 -timeout 10s -adaptiveDelay 100ms
- Results may vary depending on flag configuration, higher concurrency and lower timeouts may cause results to be incorrect due to web server behavior. The recommended timeout is 10.

### Flags

- `-url`: Base URL to scan. [REQUIRED]
- `-paths`: File containing paths to scan. [REQUIRED]
- `-concurrent`: Number of concurrent goroutines for scanning (default 10).
- `-timeout`: HTTP request timeout (default 10s).
- `-adaptiveDelay`: Initial adaptive delay between requests (default 100ms).
