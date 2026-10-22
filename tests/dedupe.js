ObjC.import('Foundation');

function run(argv) {
    var source = ObjC.unwrap($.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null));
    var windows, actions, events, hook, reads, readHook, urlChecks, assertions = 0;
    function assert(ok, label) { assertions++; if (!ok) throw Error('FAIL: ' + label); }
    function tab(id, url, options) {
        var t = {_id: id, _url: url, pinned: false, busy: false};
        Object.keys(options || {}).forEach(function (key) { t[key] = options[key]; });
        t.id = function () { return t._id; };
        t.url = function () { return t._url; };
        t.ispinned = function () { return t.pinned; };
        t.loading = function () { return t.busy; };
        t.close = function () {
            if (t.fail) throw Error('Simulated close failure');
            actions.push(id);
            windows.forEach(function (w) { w.items.forEach(function (p) { p.items = p.items.filter(function (candidate) { return candidate !== t; }); }); });
            if (t.afterClose) t.afterClose();
        };
        return t;
    }
    function recordRead(key, items) {
        reads.push({key: key, ids: items.map(function (t) { return t._id; }), phase: events.length ? events[events.length - 1].phase : ''});
        if (readHook) readHook(key, items);
    }
    function profile(name, index, items) {
        var p = {items: items, name: function () { return name; }, index: function () { return index; }};
        p.tabs = function () { return p.items; };
        ['id', 'url', 'ispinned', 'loading'].forEach(function (key) {
            p.tabs[key] = function () { recordRead(key, p.items); return p.items.map(function (t) { return t[key](); }); };
        });
        p.tabs.whose = function (query) {
            var collection = {};
            ['id', 'url', 'ispinned', 'loading'].forEach(function (key) {
                collection[key] = function () {
                    var items = p.items.filter(function (t) { return query._or.some(function (condition) { return condition.id === t._id; }); });
                    recordRead(key, items);
                    return items.map(function (t) { return t[key](); });
                };
            });
            return collection;
        };
        p.tabs.byId = function (id) {
            var t = p.items.filter(function (item) { return item._id === id; })[0];
            if (!t) throw Error('Tab disappeared');
            return t;
        };
        return p;
    }
    function window(id, profiles, active) {
        return {items: profiles, active: active || 'none', id: function () { return id; }, profiles: function () { return this.items; }, activeTab: function () { var w = this; return {id: function () { return w.active; }}; }};
    }
    var cli = new Function('Application', 'record', 'recordHost', source + '\nprogress = record; var host = webHost; webHost = function (url) { recordHost(url); return host(url); }; return run;')(function () {
        return {running: function () { return true; }, windows: function () { return windows; }};
    }, function (c, phase, window, profile, completed, total) {
        events.push({phase: phase, completed: completed || 0, total: total || 0});
        if (hook) hook(phase);
    }, function (url) { urlChecks.push(url); });
    function reset(items) { actions = []; events = []; reads = []; urlChecks = []; readHook = null; hook = null; windows = items; }
    function call(extra) {
        var c = {name: 'dedupe', dryRun: true, progress: true};
        Object.keys(extra || {}).forEach(function (key) { c[key] = extra[key]; });
        var result = JSON.parse(cli([JSON.stringify(c)]));
        var d = result.dedupe;
        assert(d.duplicates === d.closed + d.skipped + d.remaining && d.failed <= d.remaining, 'consistent tally');
        return result;
    }
    var emptySnapshot = new Function(source + '\nreturn readTabs;')()({tabs: {whose: function () { throw Error('Empty ID query'); }}}, ['url', 'ispinned'], []);
    assert(emptySnapshot.id.length === 0 && emptySnapshot.url.length === 0 && emptySnapshot.ispinned.length === 0, 'empty target IDs return empty arrays without remote calls');
    var url = 'https://example.com/path?a=1#part';
    function basic() {
        var a = tab('a', url), b = tab('b', url), c = tab('c', url);
        var p = profile('Work', 1, [a, b, c]);
        reset([window('w1', [p])]);
        return {a: a, b: b, c: c, p: p, w: windows[0]};
    }
    var fixture = basic();
    var preview = call();
    assert(preview.dedupe.groups === 1 && preview.dedupe.duplicates === 2 && preview.dedupe.remaining === 2, 'preview counts extras');
    assert(preview.output.indexOf('KEEP\tw1\ta\tWork') !== -1 && preview.output.indexOf('CLOSE\tw1\tb\tWork') !== -1 && actions.length === 0, 'default preview decisions and zero actions');
    var applied = call({dryRun: false});
    assert(applied.dedupe.closed === 2 && actions.join(',') === 'b,c' && fixture.p.items[0] === fixture.a, 'apply keeps first and closes only extras');
    assert(call({dryRun: false}).dedupe.duplicates === 0 && actions.length === 2, 'apply rerun converges with no new actions');
    assert(events.some(function (event) { return event.phase === 'closing'; }) && events.some(function (event) { return event.phase === 'closed' && event.completed === 2; }), 'close progress reflects completed writes');
    var variants = [url, url + 'x', url.replace('example', 'EXAMPLE'), url.replace('https:', 'http:'), url.replace('#part', ''), url.replace('?a=1', '?a=2'), url.replace('/path', '/path/'), 'file:///test', '', 'https://'];
    reset([window('w', [profile('Work', 1, variants.map(function (u, i) { return tab('u' + i, u); }))])]);
    assert(call().dedupe.duplicates === 0, 'full raw URL equality retains query fragment scheme host case and trailing slash differences');
    assert(!reads.some(function (read) { return read.key === 'ispinned' || read.key === 'loading'; }), 'unique URLs never read protection properties');
    assert(urlChecks.length === 0, 'unique URLs need no Foundation URL validation without domain filters');
    reset([window('w', [profile('Work', 1, [tab('f1', 'file:///x'), tab('f2', 'file:///x'), tab('blank1', ''), tab('blank2', ''), tab('bad1', 'https://'), tab('bad2', 'https://')])])]);
    assert(call({dryRun: false}).dedupe.duplicates === 0 && actions.length === 0, 'nonHTTP empty and invalid URL never close');
    fixture = basic(); fixture.b.pinned = true;
    var protectedResult = call({dryRun: false});
    assert(actions.join(',') === 'a,c' && fixture.p.items[0] === fixture.b, 'pinned keeper takes precedence over first');
    fixture = basic(); fixture.w.active = 'b'; fixture.c.pinned = true;
    protectedResult = call({dryRun: false});
    assert(actions.join(',') === 'a' && protectedResult.dedupe.skipped === 1, 'active and extra pinned copies stay');
    fixture = basic(); fixture.c.busy = true;
    assert(call({dryRun: false}).dedupe.skipped === 2 && actions.length === 0, 'one loading copy protects entire group');
    fixture = basic(); fixture.a.busy = true; fixture.b.busy = true;
    assert(call({loading: true, dryRun: false}).dedupe.skipped === 1 && actions.length === 0, 'loading filter only previews protected duplicates');
    fixture = basic(); fixture.a.pinned = true; fixture.b.pinned = true;
    assert(call({pinned: true, dryRun: false}).dedupe.skipped === 1 && actions.length === 0, 'pinned filter never closes protected tabs');
    fixture = basic(); fixture.b.pinned = true;
    assert(call({unpinned: true}).dedupe.duplicates === 1 && actions.length === 0, 'unpinned duplicate preview filters before grouping without closing');
    assert(call({unpinned: true, dryRun: false}).dedupe.closed === 1 && actions.join(',') === 'c' && fixture.p.items.indexOf(fixture.b) !== -1, 'unpinned dedupe preserves pinned copies');
    fixture = basic();
    assert(call({excludeDomains: [{host: 'example.com'}], keywords: ['example'], dryRun: false}).dedupe.duplicates === 0 && actions.length === 0, 'dedupe exclusions veto keyword matches before grouping');
    fixture = basic(); windows[0].items.push(profile('Other', 2, [fixture.b]));
    protectedResult = call({profile: 'Work', dryRun: false});
    assert(actions.join(',') === 'a,c' && fixture.p.items[0] === fixture.b, 'shared keeper protected even outside selected Profile');
    fixture = basic(); windows[0].items.push(profile('Other', 2, [fixture.b, fixture.c]));
    protectedResult = call({profile: 'Work', dryRun: false});
    assert(actions.join(',') === 'a' && protectedResult.dedupe.skipped === 1, 'extra shared copy remains and physical duplicate counted once');
    var x = tab('x', url), y = tab('y', url);
    reset([window('w', [profile('Work', 1, [x, y]), profile('Work', 2, [x, y])])]);
    protectedResult = call({dryRun: false});
    assert(protectedResult.dedupe.groups === 2 && protectedResult.dedupe.duplicates === 1 && protectedResult.dedupe.skipped === 1 && actions.length === 0, 'shared duplicate physical identity counted once across Profile groups');
    reset([window('w', [profile('Work', 1, [tab('a', url)]), profile('Work', 2, [tab('b', url)])]), window('other', [profile('Work', 1, [tab('c', url)])])]);
    assert(call({dryRun: false}).dedupe.groups === 0 && actions.length === 0, 'same named Profiles and different windows never mix');
    fixture = basic();
    assert(call({domains: [{host: 'absent.test'}]}).dedupe.duplicates === 0, 'strict domain filter');
    assert(call({domains: [{host: 'absent.test'}], keywords: ['PATH?A=1']}).dedupe.duplicates === 2, 'domain keyword union with literal case insensitive URL text');
    assert(call({domains: [{host: 'example.com', exact: true}]}).dedupe.duplicates === 2, 'exact domain accepts root');
    assert(call({domains: [{host: 'example.com', subdomains: true}]}).dedupe.duplicates === 0, 'wildcard excludes root');
    var error = '';
    try { call({profile: 'Missing'}); } catch (e) { error = String(e); }
    assert(error.indexOf('No matching source profile') !== -1, 'missing Profile gives clear error');
    error = '';
    try { call({window: 'absent'}); } catch (e) { error = String(e); }
    assert(error.indexOf('Window not found') !== -1, 'window filter rejects unknown ID');
    function preflight(label, mutate) {
        fixture = basic();
        hook = function (phase) { if (phase === 'plan') mutate(fixture); };
        var result = call({dryRun: false});
        assert(actions.length === 0 && result.error && result.dedupe.failed === 1 && result.dedupe.remaining === 2, label + ' before all writes');
    }
    preflight('keeper removed', function (f) { f.p.items.shift(); });
    preflight('keeper navigated', function (f) { f.a._url += '/new'; });
    preflight('candidate navigated', function (f) { f.c._url += '/new'; });
    preflight('candidate added membership', function (f) { f.w.items.push(profile('New', 2, [f.b])); });
    preflight('keeper added membership', function (f) { f.w.items.push(profile('New', 2, [f.a])); });
    preflight('candidate became active', function (f) { f.w.active = 'c'; });
    preflight('candidate became pinned', function (f) { f.c.pinned = true; });
    preflight('candidate started loading', function (f) { f.c.busy = true; });
    preflight('keeper started loading', function (f) { f.a.busy = true; });
    fixture = basic(); fixture.c.fail = true;
    var partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.dedupe.closed === 1 && partial.dedupe.remaining === 1 && partial.dedupe.failed === 1 && partial.error.indexOf('Simulated close failure') !== -1, 'close failure retains partial tally and original cause');
    fixture.c.fail = false;
    assert(call({dryRun: false}).dedupe.closed === 1 && fixture.p.items.length === 1, 'partial rerun closes remaining extra only');
    fixture = basic(); fixture.b.afterClose = function () { fixture.c._url += '/changed'; };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.dedupe.remaining === 1 && partial.error.indexOf('changed') !== -1, 'midbatch candidate navigation stops with partial result');
    fixture = basic(); fixture.b.afterClose = function () { fixture.a.busy = true; };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.error.indexOf('loading') !== -1, 'newly loading keeper stops subsequent close');
    fixture = basic();
    var fourth = tab('d', url); fixture.p.items.push(fourth);
    fixture.b.afterClose = function () { fourth.busy = true; };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.dedupe.remaining === 2 && partial.error.indexOf('loading') !== -1, 'new loading other group member protects group before next candidate');
    fixture = basic(); fixture.b.afterClose = function () { fixture.w.active = 'c'; };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.error.indexOf('protected') !== -1, 'new active candidate never closes');
    fixture = basic(); fixture.b.afterClose = function () { fixture.w.items.push(profile('Other', 2, [fixture.c])); };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.error.indexOf('membership') !== -1, 'new shared candidate never closes');
    fixture = basic();
    var second = profile('Work', 1, [tab('later1', url), tab('later2', url)]);
    windows.push(window('later', [second]));
    hook = function (phase) { if (phase === 'plan') second.items[1]._url += '/new'; };
    partial = call({dryRun: false});
    assert(actions.length === 0 && partial.dedupe.remaining === 3, 'later window preflight invalidation prevents earlier window closes');
    fixture = basic();
    fixture.b.afterClose = function () { fixture.w.items.push(profile('New', 2, [fixture.a])); };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b' && partial.error.indexOf('membership') !== -1, 'new Profile sharing keeper stops subsequent close');
    fixture = basic();
    var unrelated = tab('unrelated', 'https://elsewhere.test');
    fixture.p.items.push(unrelated);
    fixture.b.afterClose = function () {
        unrelated._url += '/navigation';
        fixture.p.items.reverse();
        fixture.p.items.push(tab('new-unrelated', 'https://new.test'));
    };
    partial = call({dryRun: false});
    assert(actions.join(',') === 'b,c' && !partial.error, 'unrelated addition navigation and reorder do not block valid close');
    assert(!reads.some(function (read) { return (read.phase === 'closing' || read.phase === 'closed') && read.key !== 'id' && read.ids.some(function (id) { return id === 'unrelated' || id === 'new-unrelated'; }); }), 'per-close property reads exclude unrelated tabs');
    fixture = basic();
    fixture.p.items.push(tab('d', url + '/second'), tab('e', url + '/second'));
    for (var n = 0; n < 20; n++) fixture.p.items.push(tab('unique' + n, 'https://unique.test/' + n));
    partial = call({dryRun: false});
    var preflightURLs = reads.filter(function (read) { return read.phase === 'plan' && read.key === 'url'; });
    assert(partial.dedupe.closed === 3 && preflightURLs.length === 1 && preflightURLs[0].ids.length === 5, 'many duplicate groups share one per-source window preflight');
    assert(reads.filter(function (read) { return read.key === 'ispinned' || read.key === 'loading'; }).every(function (read) { return read.ids.every(function (id) { return id.indexOf('unique') !== 0; }); }), 'unrelated unique tabs never need protection properties');
    var propertyItems = reads.reduce(function (sum, read) { return sum + (read.key === 'id' ? 0 : read.ids.length); }, 0);
    assert(propertyItems < 150, '25 tabs two groups three closes stay below two full property passes');
    fixture = basic();
    var stateURLReads = 0;
    readHook = function (key, items) {
        if (key === 'url' && ++stateURLReads === 2) fixture.b._url += '/changed';
    };
    error = '';
    try { call({dryRun: false}); } catch (e) { error = String(e); }
    assert(actions.length === 0 && error.indexOf('changed') !== -1, 'navigation between initial grouping and protection snapshot never closes');
    fixture = basic();
    stateURLReads = 0;
    readHook = function (key, items) {
        if (key === 'url' && ++stateURLReads === 2) fixture.p.items.reverse();
    };
    error = '';
    try { call({dryRun: false}); } catch (e) { error = String(e); }
    assert(actions.length === 0 && error.indexOf('Tabs changed while reading') !== -1, 'targeted property snapshot rejects same-size ID reorder');
    reset([window('empty', [profile('Work', 1, [])])]);
    windows[0].activeTab = function () { throw Error('No active tab'); };
    assert(call({dryRun: false}).dedupe.duplicates === 0, 'empty Profiles need no active tab');
    reset([]);
    assert(call({dryRun: false}).dedupe.duplicates === 0 && actions.length === 0, 'empty browser returns safe no-op');
    return assertions + ' dedupe assertions passed';
}
