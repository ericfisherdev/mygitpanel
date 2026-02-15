// Attach CSRF token from cookie to all mutating HTMX requests as a header.
document.addEventListener('htmx:configRequest', function(event) {
    var method = (event.detail.verb || '').toUpperCase();
    if (method === 'POST' || method === 'DELETE' || method === 'PUT' || method === 'PATCH') {
        var match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
        if (match) {
            event.detail.headers['X-CSRF-Token'] = decodeURIComponent(match[1]);
        }
    }
});
