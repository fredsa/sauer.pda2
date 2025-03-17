function spin(event) {
	const submit = document.getElementById('submit');
	submit.disabled = true;
	console.log(submit);
	const spinner = "\\|/-";
	let count = 0;
	setInterval(() =>  {
		submit.value = "Searching… " + spinner[count%spinner.length];
		count++;
	}, 100);
}

function highlight() {
	var regex;
	var color;
	function highlightNode(node) {
		if (node.nodeType === Node.TEXT_NODE) {
			const newNodes = [];
			let lastIndex = 0;
			let match;

			while ((match = regex.exec(node.nodeValue)) !== null) {
				const span = document.createElement('span');
				span.style.backgroundColor = color;
				span.textContent = match[0];

				newNodes.push(
					document.createTextNode(node.nodeValue.substring(lastIndex, match.index)),
					span
				);
				lastIndex = regex.lastIndex;
			}

			if (newNodes.length > 0) {
				newNodes.push(document.createTextNode(node.nodeValue.substring(lastIndex)));
				// Replace the original text node with the new array of nodes.
				node.replaceWith(...newNodes);
			}

		} else if (node.nodeType === Node.ELEMENT_NODE) {
			// Recursively highlight child nodes.
			node.childNodes.forEach(highlightNode);
		}
	}

	const q = new URLSearchParams(window.location.search).get('q');
	if (!q || !q.trim()) {
		return;
	}

	// Escape special regex characters in the search term.
	const escapedQ = q.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&');

	regex = new RegExp('\\b' + escapedQ, 'gi');
	color = 'yellow';
	highlightNode(document.body);

	regex = new RegExp('(?<!\\b)' + escapedQ, 'gi');
	color = 'orange';
	highlightNode(document.body);
}

document.addEventListener('DOMContentLoaded', function () {
	document.searchform.q.focus();
	document.searchform.q.select();
	highlight();
});