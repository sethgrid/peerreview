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

### Contributing

To keep the deploy of `peerreview` simple, you must bundle all the required files (html, css, javascript).
If the schema is changed, we have to adjust the schema version variable to prevent strange run time errors on queries. There is no current live migration strategy.

### Requirements
Go1.8.1+ : there is an error in earlier versions for sqlite3. See https://github.com/golang/go/issues/19734.

For the app to function, you will have to hook up Google Sign-In. See above.