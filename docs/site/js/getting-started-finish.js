function domain_update() {
    const exampleDomainName = /%s\.example\.com/ig
    const exampleDomainSuffix = /example\.com/ig;
    let domainPattern = sessionStorage.getItem('dhctl-domain');
    let domainSuffix = domainPattern ? domainPattern.replace('%s\.', '') : null;

    if (domainSuffix && domainSuffix.length > 0) {
        $('code').filter(function () {
            return ((this.innerText.match('admin@example.com') || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                $(this)[0].innerText = content.replace('admin@example.com', 'admin@' + domainPattern.replace(/%s[^.]*./, ''));
            }
        });

        $('a').filter(function () {
            return ((this.textContent.match(exampleDomainSuffix) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].textContent : null;
            let href = $(this).attr('href')
            if (content && content.length > 0) {
                $(this).attr('href', href.replace(/([\S]+)\.example\.com/i, domainPattern.replace('%s', href.match(/([\S]+)\.example\.com/i)[1])));
                $(this)[0].textContent = content.replace(/([\S]+)\.example\.com/i, domainPattern.replace('%s', content.match(/([\S]+)\.example\.com/i)[1]));
            }
        });
    }
}

$(document).ready(function () {
    domain_update();
    generate_password();
    replace_snippet_password();
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code');
});
