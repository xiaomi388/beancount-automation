<html>
<body>
<button id='linkButton'>Open Link - Institution Select</button>
<p id="results"></p>
<script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
<script>
var linkHandler = Plaid.create({
token: '{{ .LinkToken }}',
onLoad: function() {
// The Link module finished loading.
},
onSuccess: function(public_token, metadata) {
// Send the public_token to your app server here.
// The metadata object contains info about the institution the
// user selected and the account ID, if selectAccount is enabled.
console.log('public_token: '+public_token+', metadata: '+JSON.stringify(metadata));
document.getElementById("results").innerHTML = "public_token: " + public_token + "<br>metadata: " + metadata;
},
onExit: function(err, metadata) {
// The user exited the Link flow.
if (err != null) {
// The user encountered a Plaid API error prior to exiting.
}
// metadata contains information about the institution
// that the user selected and the most recent API request IDs.
// Storing this information can be helpful for support.
}
});
// Trigger the standard institution select view
document.getElementById('linkButton').onclick = function() {
linkHandler.open();
};
</script>
</body>
</html>
