package res

const ReloadScript = `
let timer = null;
let ws = null;
function connect() {
	console.log('Connecting...');

	try {
		ws = new WebSocket("ws://" + window.location.host + "/hotreload-go/ws");

		ws.onopen = function() {
			console.log('Connected');
			if(timer != null) {
				clearInterval(timer);
				timer = null;
			}
		}

		ws.onclose = function() {
			console.log('Disconnected');
			ws = null;
			if(timer == null) {
				timer = setInterval(connect, 1000);
			}
		}

		ws.onmessage = function(e) {
			msg = JSON.parse(e.data);
			console.log('Source file updated: ' + msg.Path);
			if (msg.autoReload) {
				console.log('Reloading page');
				window.location.reload();
			}
		}

		ws.onerror = function(e) {
			console.log('Connection failed');
			if(timer == null) {
				timer = setInterval(connect, 1000);
			}
		}

	} catch(err) {
		console.log('Connection failed');
		if(timer == null) {
			timer = setInterval(connect, 1000);
		}
		return;
	}

}

connect();
`
