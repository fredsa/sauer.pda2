package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"google.golang.org/appengine/v2/user"
)

func renderPremable(w io.Writer, ctx context.Context, q string) {
	u := user.Current(ctx)
	if u == nil && isDev() {
		u = &user.User{Email: "someone@gmail.com"}
	}

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1.0">
		<title>PDA2GO</title>
		<style type="text/css">
			input[type=text],option,textarea {
				background-color: #eff9fb;
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
			.title {
				text-decoration: none;
				font-size: 2em;
				font-weight: bold;
				padding: 0.2em 0em 0.5em;
				display: block;
				color: black;
			}
			.title .go {
				color: darkgreen;
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
			<a href="/" class="title">PDA2<span class="go">GO</span></a></a>
			<div class="email">%s</div>
			<form name="searchform" method="get">
				<input type="text" name="q" value="%s"> <input type="submit" value="Search"><br>
			</form>

			<hr>
	`,
		u.Email,
		q)

	renderCreateEntity(w, "Person", nil)
	fmt.Fprintf(w, `<br><br>`)
}

func renderPostamble(ctx context.Context, w io.Writer) {
	if isAdmin(ctx) {
		fmt.Fprintf(w, `
			<br>
			<div class="admin"><a href="%s" target="_blank" >Console</a></div>
			<div class="admin"><a href="/mailmerge">mailmerge.csv</a></div>
			<div class="admin"><a href="/task/notify">/task/notify</a></div>
			<div class="admin danger"><a href="/task/fix/all/" onclick="return prompt('Enter CONFIRM to continue:') == 'CONFIRM'">/task/fix/all/</a></div>
		`, consoleURL())
	}

	fmt.Fprintf(w, `
		<script>
			document.searchform.q.focus();
			document.searchform.q.select();
		</script>
	`)

	fmt.Fprintf(w, `
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
		os.Getenv(GAE_RUNTIME))
}
