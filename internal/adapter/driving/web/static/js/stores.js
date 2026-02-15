// Alpine.js store initialization.
// This file MUST be loaded with defer AFTER alpine-persist and BEFORE alpine core
// so that the alpine:init listener is registered in the correct order.
document.addEventListener('alpine:init', function() {
    Alpine.store('theme', {
        dark: Alpine.$persist(false).as('darkMode')
    });
});
