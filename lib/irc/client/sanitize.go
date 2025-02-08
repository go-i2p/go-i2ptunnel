package ircclient

/** TODO: implement filters from Java I2P using IRC Filter from go-connfilter.
Java filters copied here:

// Replaces real hostnames with .i2p addresses
if (line.startsWith("PING ")) {
    line = "PING " + _webircHost;
} else if (hostPattern.matcher(line).find()) {
    line = line.replaceAll("([:.])[-a-zA-Z0-9.]+(\\.[a-zA-Z]{2,})", "$1" + _webircHost);
}

// Blocks unsafe CTCP commands
if (line.indexOf("\001") >= 0) {
    // CTCP filtering
    if (line.toUpperCase().indexOf("DCC") >= 0)
        return null;
}

// DCC command protection
private void filterDCC(String line) {
    if (line.toUpperCase().startsWith(":DCC ")) {
        // Block DCC
        return null;
    }
}

// Masks real user identities
private String filterUserHost(String line) {
    if (userPattern.matcher(line).matches()) {
        return line.replaceAll("![^@]+@[^ ]+", "!user@" + _webircHost);
    }
    return line;
}
*/
