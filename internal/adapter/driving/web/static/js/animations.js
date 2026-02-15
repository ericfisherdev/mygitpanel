// GSAP animations triggered by HTMX swap events (GUI-06).
document.addEventListener('htmx:afterSwap', function(event) {
    var target = event.detail.target;
    if (target.id === 'pr-detail') {
        gsap.from('#pr-detail > *', {
            opacity: 0, y: 20, duration: 0.3, stagger: 0.05, ease: 'power2.out'
        });
    }
    if (target.id === 'pr-list') {
        gsap.from('#pr-list > *', {
            opacity: 0, x: -10, duration: 0.2, stagger: 0.03, ease: 'power1.out'
        });
    }
});
