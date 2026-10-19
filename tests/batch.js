ObjC.import('Foundation');

function run(argv) {
    var source = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null);
    if (!source) throw Error('Cannot read CLI source');
    var actions = [];
    function tab(id, title, url) {
        return {id: function () { return id; }, title: function () { return title; }, url: function () { return url; },
            move: function (options) {
                if (this.fail) throw Error('Simulated move failure');
                actions.push(id + '>' + options.to.name());
                if (options.to.items.indexOf(this) === -1) options.to.items.push(this);
            }};
    }
    function profile(name, items) {
        var p = {name: function () { return name; }, index: function () { return 1; }, items: items};
        function tabs() { return items; }
        ['id', 'title', 'url'].forEach(function (prop) { tabs[prop] = function () { return items.map(function (t) { return t[prop](); }); }; });
        tabs.byId = function (id) { return items.filter(function (t) { return t.id() === id; })[0]; };
        p.tabs = tabs;
        return p;
    }
    function window(id, profiles) { return {id: function () { return id; }, profiles: function () { return profiles; }}; }
    var a = tab('a', 'Example', 'https://EXAMPLE.COM:8443/path');
    var b = tab('b', 'Sub', 'https://a.b.example.com/');
    var shared = tab('shared', 'Shared', 'http://example.com/');
    var spoof = tab('spoof', 'example.com in title', 'https://example.com.evil.test/example.com');
    var path = tab('path', 'Path', 'https://other.test/example.com');
    var x = tab('x', 'Second window', 'http://example.com/');
    var work1 = profile('Work', [shared]);
    var personal1 = profile('Personal', [a, b, shared, spoof, path]);
    var personal2 = profile('Personal', [x]);
    var windows = [window('w1', [work1, personal1]), window('w2', [personal2])];
    var cli = new Function('Application', ObjC.unwrap(source) + '\nreturn run;')(function (name) {
        if (name !== 'Dia') throw Error('Wrong app');
        return {running: function () { return true; }, windows: function () { return windows; }};
    });
    var assertions = 0;
    function assert(ok, label) { assertions++; if (!ok) throw Error('FAIL: ' + label); }
    function call(c) { return JSON.parse(cli([JSON.stringify(c)])); }
    function error(c, fragment) {
        var message = '';
        try { call(c); } catch (e) { message = String(e); }
        assert(message.indexOf(fragment) !== -1, 'expected error ' + fragment + ', got ' + message);
    }
    var exact = [{host: 'example.com', subdomains: false}];
    var union = [{host: 'example.com', subdomains: false}, {host: 'example.com', subdomains: true}];
    assert(call({name: 'count', profile: 'Personal', domains: exact}).output === '3', 'exact host, source filter before shared-tab dedupe');
    assert(call({name: 'count', profile: 'Personal', domains: [{host: 'example.com', subdomains: true}]}).output === '1', 'wildcard excludes root and includes nested subdomain');
    assert(call({name: 'count', profile: 'Personal', domains: union}).output === '4', 'multiple domains use union and ignore title/path/spoof suffix');
    assert(call({name: 'links', profile: 'Personal', domains: exact}).output.split('\n').length === 3, 'same URL in two windows retained');
    assert(call({name: 'profiles', counts: true, window: 'w1', domains: exact}).output.indexOf('Work\t1') !== -1, 'profile counts use actual membership');
    assert(call({name: 'profiles', counts: true, window: 'w1', domains: exact}).output.indexOf('Personal\t2') !== -1, 'shared tab counts in each profile');
    error({name: 'count', profile: 'Missing'}, 'No matching source profile');
    var request = {name: 'move-domain', profile: 'Personal', domains: union, target: 'Work', dryRun: true};
    error(request, 'No matching profile name in window w2');
    assert(actions.length === 0, 'all-window preflight makes zero writes');
    var work2 = profile('Work', []);
    windows[1] = window('w2', [work2, personal2]);
    var preview = call(request);
    var dry = preview.move;
    assert(dry.matched === 4 && dry.moved === 0 && dry.skipped === 1 && dry.failed === 0 && dry.remaining === 3, 'dry-run plan');
    assert(preview.output.indexOf('MOVE\tw1\ta\tPersonal\tWork\thttps://EXAMPLE.COM:8443/path') !== -1, 'preview identifies move and URL');
    assert(preview.output.indexOf('SKIP\tw1\tshared\tPersonal\tWork\thttp://example.com/') !== -1, 'preview identifies target membership');
    assert(preview.output.indexOf('spoof') === -1 && preview.output.indexOf('path\t') === -1, 'preview excludes title, path and suffix spoof');
    assert(actions.length === 0, 'dry-run zero actions');
    assert(call({name: 'move-domain', domains: [{host: 'absent.test'}], target: 'Work', dryRun: true}).output.indexOf('No matching tabs') !== -1, 'empty preview is explicit');
    request.dryRun = false;
    b.fail = true;
    var failed = call(request);
    assert(failed.move.moved === 1 && failed.move.failed === 1 && failed.move.remaining === 2, 'partial failure accounting');
    assert(failed.error.indexOf('Simulated move failure') !== -1 && failed.error.indexOf('w1') !== -1 && failed.error.indexOf('b') !== -1 && failed.error.indexOf('Work') !== -1 && actions.join(',') === 'a>Work', 'failure identifies tab and stops');
    b.fail = false;
    var retry = call(request).move;
    assert(retry.matched === 4 && retry.moved === 2 && retry.skipped === 2 && retry.remaining === 0, 'retry skips actual target membership');
    assert(actions.join(',') === 'a>Work,b>Work,x>Work', 'moves stay within source windows');
    error({name: 'move-domain', profile: 'Personal', domains: union, target: 'work'}, 'No matching profile');
    assert(actions.length === 3, 'missing target never writes');
    windows[1] = window('w2', [work2, profile('Work', []), personal2]);
    error(request, 'Ambiguous profile name in window w2');
    assert(actions.length === 3, 'ambiguous later window never writes');
    return assertions + ' batch assertions passed';
}
