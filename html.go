package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/user"
)

func preamble(ctx context.Context, q string) string {
	var buffer bytes.Buffer

	u := user.Current(ctx)
	if u == nil && isDev() {
		u = &user.User{Email: "someone@gmail.com"}
	}

	clazz := ""
	if projectID() == "sauer-pda" && !strings.HasPrefix(appengine.DefaultVersionHostname(ctx), "sauer-pda"+".") {
		clazz = "warn"
	}

	buffer.WriteString(strings.TrimSpace(fmt.Sprintf(`
<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1.0">
		<title>PDA2GO</title>
		<link rel="icon" href="favicon.ico" type="image/x-icon">
		<link rel="stylesheet" href="/static/main.css">
		<script src="/static/main.js"></script>
	</head>
	<body class="pda">
		<span class="title">	
			<a href="/">PDA2<span class="go">GO</span></a>
			<span class="appid %s">%s</span></a>
		</span>
		<div class="email">%s</div>
		<form name="searchform" method="get" onsubmit="spin()">
			<input type="text" name="q" autocomplete="off" value="%s">
			<input type="submit" id="submit" value="Search"><br>
		</form>

		<hr>
	`,
		clazz,
		projectID(),
		u.Email,
		q)))

	buffer.WriteString(createEntityLink("Person", nil))
	buffer.WriteString(`<br><br>`)

	return buffer.String()
}

func postamble(ctx context.Context) string {
	var buffer bytes.Buffer

	if isAdmin(ctx) {

		buffer.WriteString(fmt.Sprintf(`
			<br>
			<div class="admin"><a href="%s" target="_blank">Console</a>, <a href="%s" target="_blank">Datastore</a></div>
			<div class="admin"><a href="/mailmerge">mailmerge.csv</a></div>
			<div class="admin"><a href="/task/notify">/task/notify</a></div>
			<div class="admin danger"><a href="/task/fix/all/" onclick="return prompt('Enter CONFIRM to continue:') == 'CONFIRM'">/task/fix/all/</a></div>
		`,
			consoleURL(),
			datastoreURL(),
		))
	}

	buffer.WriteString(fmt.Sprintf(`
		<div class="powered">
			version <span class="version">%s</span>,
			powered by Go on App Engine
			(%s %s)
		</div>
	</body>
</html>
    `,
		os.Getenv(GAE_VERSION),
		os.Getenv(GAE_ENV),
		os.Getenv(GAE_RUNTIME)))

	return buffer.String()
}
