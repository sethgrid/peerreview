# Peer Review
### 360° Anonymous Feedback

`peerreview` is a simple web app deplayed via a single executable binary. It is a peer review system allowing for 360° anonymous feedback on strengths and opportunities for growth.

### Installation / Running

Build instructions:
`peerreview` leverages sqlite3 as the data persistence layer. To avoid needing to use cgo to build this service, you can `go install github.com/mattn/go-sqlite3`.

Assuming you've installed `go-sqlite3`:
```
go build
./peerreview
```

By default, `peerreview` will create `peerreivew.db` in the working directory. This can adjusted via command line flags.

You will need to set up Google Sign-In.

#### Google Sign-in

1) You will need to create your own [Google API Console project and client ID](https://developers.google.com/identity/sign-in/web/devconsole-project). If you are developing locally, be sure to set up your credential to work with `http://localhost:3333`.

2) Update your client id in `web/static/index.html`

3) Download the .json config from your [Google API Console Credentials page](https://console.developers.google.com/apis/credentials) and move it to `oauth_config.json`.

### API

When a user signs in via Google Sign-In, there is a cookie created called `auth`, eg: `auth=XhfsnkIJPRwe_znXfhizqkVBtoD.AeXYVcRa`. This session token is stored in memory server side with an expiration of 24 hours. This same session token can be used to make API calls client side into the system using the `X-Session-Token` header. For example, `curl localhost:3333/dash --header "X-Session-Token: XhfsnkIJPRwe_znXfhizqkVBtoD.AeXYVcRa` and `curl localhost:3333/dash --cookie "auth=XhfsnkIJPRwe_znXfhizqkVBtoD.AeXYVcRa"` both will work. Any authenticated endpoint will check for either a valid auth cookie or x-session-token.

### Contributing

To keep the deploy of `peerreview` simple, you must bundle all the required files (html, css, javascript).
If the schema is changed, we have to adjust the schema version variable to prevent strange run time errors on queries. There is no current live migration strategy.

### Debugging

When running tests, if you get an unexpected result back from the db, you can inspect the db, such as:

```
$ go test -save-db -run TestAPIUserGoal
Using seed 1499699308
Optional test flags: -randseed :int -save-db :bool -show-logs :bool

2017/07/10 08:08:28 keeping db .test_db_1499699308_17
--- FAIL: TestAPIUserGoal (0.01s)
	main_test.go:165: got goal "", expected "I want to make awesome things"
FAIL
exit status 1
FAIL	github.com/sethgrid/peerreview	0.042s
08:08:28 sethammons@sammons:~/workspace/go/src/github.com/sethgrid/peerreview (git:master*:404f9cd)
$ sqlite3 .test_db_1499699308_17
SQLite version 3.16.0 2016-11-04 19:09:39
Enter ".help" for usage hints.
sqlite> select * from users;
1|Test User|.test_db_1499699308_17@example.com|I want to make awesome things
```

In this example, we can see that the test case has trouble getting the `goals` value back from the user table, but we can see that it is indeed in the db. This gives us a jumping off point for debugging.

### GUI / Web Frontend

This project leverages create-react-app and was initialized with `cd web && create-react-app peer-review`. You can run the web frontend with `cd web/peer-review && PORT=3333 yarn start`. You will need to start the Go API with port 3000.

### API Requirements
Go1.8.1+ : there is an error in earlier versions for sqlite3. See https://github.com/golang/go/issues/19734.

For the app to function, you will have to hook up Google Sign-In. See above.

### TODO
  - ~~set up sqlite3 as the backing datastore~~
  - ~~set up schema versioning~~
  - ~~set up google sign-in with ability to log in and log out~~
  - ~~set up alternate auth mw handling for bearer token that has the same content as the auth cookie value. Add documentation for curling with this~~
  - ~set up endpoints for all interactions~
  - set up port handling and google client secret handling so this can be set in one place and used in both back and front ends
  - set up /dash to be a single page application
  - set up vendoring of web directory into binary
  - vendor Go dependencies