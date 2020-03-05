# gotestsum [original]

`gotestsum` runs tests, prints friendly test output and a summary of the test run.  Requires Go 1.10+.

For the original documentation see : [gotestsum](pkg/gotestsum/README.md) docs 
 [![GoDoc](https://godoc.org/gotest.tools/gotestsum?status.svg)](https://godoc.org/gotest.tools/gotestsum)

# gotestsum [modified]

This is a modified version of the original gotestsum who's purpose is to manage github issues related to failing tests.
It acts like a wrapper over the original gotestsum and it extends the functionality by enabling to open and close github
issues relating to failing tests.

## Github Integration

In order to interact with github [go-github](https://github.com/google/go-github) library is used. A [personal API token](https://github.blog/2013-05-16-personal-api-tokens/) 
must be given in order to interact with github repositories. Example :

```
gotestsum 
--post 
--token="your token here" 
--owner="astralkn" 
--repo="gotestsum"
```

Github flags:
 * `post`  - create and close issues based on the tests results.
 * `token` - requiers personal API token in order to create and modify github issues.
 * `owner` - github repository owner.
 * `repo`  - github repository.
