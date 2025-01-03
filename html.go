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

	buffer.WriteString(fmt.Sprintf(`
<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1.0">
		<title>PDA2GO</title>
		<style type="text/css">
			input[type=text],textarea,option:not(:checked) {
				background-color: #eff9fb;
			}
			option:checked {
				background-color: #86c7fe;
			}
			input[type=text],textarea {
				width: 30em;
				max-width: calc(100%% - 8px);
			}
			textarea {
				width: 50em;
				height: 20em;
				font-family: monospace;
			}
			body {
				line-height: 1.3em;
				background-color: #e7f4ff;
			}
			.comments {
				font-family: monospace;
				color: #c44;
				white-space: pre;
				padding-bottom: 2em;
			}
			.tag {
				font-size: small;
			}
			.indent {
				padding-left: 2em;
			}
			.edit-link {
				font-size: small;
				background-color: #ddd;
				margin: 0.2em;
				padding: 0px 10px;
				border-radius: 5px;
				display: inline-block;
			}
			.thing {
				font-weight: bold;
				margin: 0em 0.5em 0em 0.2em;
			}
			.thing.Address {
				color: green;
			}
			.thing.Contact {
				color: blue;
			}
			.thing.Calendar {
				color: purple;
			}
			.title a {
				text-decoration: none;
				font-size: 2em;
				font-weight: bold;
				padding: 0.2em 0em 0.5em;
				color: black;
			}
			.title a .go {
				color: darkgreen;
			}
			.appid {
				font-family: monospace;
				background: lightblue;
			}
			.appid.warn {
				background: red;
				color: white;
				padding: 0 24px;
			}
			.powered {
				color: #777;
				font-style: italic;
				text-align: center;
			}
			.email {
				position: absolute;
				right: 0.5em;
				top: 0.2em;
				color: #444;
			}
			.disabled, .disabled * {
				color: #ccc !important;
			}
			.admin a, .admin a:visited {
				color: #844;
			}
			.danger {
				margin-top: 1em;
			}
			.danger a, .danger a:visited {
				color: #f00;
			}
			.version {
				color: #800;
				font-family: monospace;
			}
		</style>
		</head>
		<body class="pda">
			<span class="title">	
				<a href="/">PDA2<span class="go">GO</span></a>
				<span class="appid %s">%s</span></a>
			</span>
			<div class="email">%s</div>
			<form name="searchform" method="get">
				<input type="text" name="q" autocomplete="off" value="%s"> <input type="submit" value="Search"><br>
			</form>

			<hr>
	`,
		clazz,
		projectID(),
		u.Email,
		q))

	buffer.WriteString(entityLink("Person", nil))
	buffer.WriteString(fmt.Sprintf(`<br><br>`))

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
		<script>
			document.searchform.q.focus();
			document.searchform.q.select();
		</script>
	`))

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
