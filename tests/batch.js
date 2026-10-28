ObjC.import('Foundation');

function run(argv) {
    var source = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null);
    if (!source) throw Error('Cannot read CLI source');
    var actions = [], events = [], transportFailure = false, scalarReads = 0, vectorRead = false;
    function tab(id, title, url) {
        return {id: function () { if (!vectorRead) scalarReads++; return id; }, title: function () { return title; }, url: function () { if (!vectorRead) scalarReads++; return url; }, ispinned: function () { return !!this.pinned; }, loading: function () { return !!this.isLoading; },
            move: function (options) {
                if (this.fail) throw Error('Simulated move failure');
                actions.push(id + '>' + options.to.name());
                if (options.to.items.indexOf(this) === -1) options.to.items.push(this);
            }};
    }
    function profile(name, items) {
        var p = {name: function () { return name; }, index: function () { return 1; }, items: items};
        p.reads = 0;
        function collection(ids) {
            function tabs() { return items; }
            ['id', 'title', 'url', 'ispinned', 'loading'].forEach(function (prop) { tabs[prop] = function () {
                p.reads++;
                if (p.onRead) p.onRead(prop, ids);
                vectorRead = true;
                try {
                    return items.filter(function (t) { return !ids || ids.indexOf(String(t.id())) !== -1; }).map(function (t) { return t[prop](); });
                } finally { vectorRead = false; }
            }; });
            tabs.byId = function (id) {
                vectorRead = true;
                try { return items.filter(function (t) { return t.id() === id; })[0]; }
                finally { vectorRead = false; }
            };
            tabs.whose = function (query) {
                if (query._or && query._or.length < 2) throw Error('Whose clause "_or" array requires at least two elements.');
                return collection((query._or || [query]).map(function (condition) { return String(condition.id); }));
            };
            return tabs;
        }
        var tabs = collection();
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
    var cli = new Function('Application', 'recordProgress', ObjC.unwrap(source) + '\nprogress = recordProgress; return run;')(function (name) {
        if (name !== 'Dia') throw Error('Wrong app');
        return {running: function () { return true; }, windows: function () { return windows; }};
    }, function (c, phase, window, profile, completed, total) {
        if (!c.progress) return;
        events.push({phase: phase, window: window, profile: profile, completed: completed || 0, total: total || 0, actions: actions.length});
        if (transportFailure && phase === 'moved') throw Error('Progress transport failure');
    });
    var assertions = 0;
    function assert(ok, label) { assertions++; if (!ok) throw Error('FAIL: ' + label); }
    function call(c) { events = []; return JSON.parse(cli([JSON.stringify(c)])); }
    function error(c, fragment) {
        var message = '';
        try { call(c); } catch (e) { message = String(e); }
        assert(message.indexOf(fragment) !== -1, 'expected error ' + fragment + ', got ' + message);
    }
    var exact = [{host: 'example.com', subdomains: false, exact: true}];
    assert(call({name: 'count', profile: 'Personal', domains: [{host: 'example.com'}]}).output === '4', 'plain domain includes root and all subdomain levels');
    var union = [{host: 'example.com', exact: true}, {host: 'example.com', subdomains: true}];
    var matcher = new Function(ObjC.unwrap(source) + '\nreturn domainMatch;')();
    ['https://example.com', 'http://a.example.com', 'https://a.b.example.com:8443/path', 'https://EXAMPLE.COM./'].forEach(function (url) { assert(matcher(url, [{host: 'example.com'}]), 'plain accepts ' + url); });
    ['https://notexample.com', 'https://example.com.evil.test', 'https://other.test/example.com', 'file://example.com', 'https://example.com@evil.test', 'https://evil.test/?q=example.com'].forEach(function (url) { assert(!matcher(url, [{host: 'example.com'}]), 'plain excludes ' + url); });
    assert(!matcher('https://a.example.com', exact), 'exact excludes descendants');
    assert(matcher('https://other.test', [{host: 'example.com'}, {host: 'other.test'}]), 'union of distinct domains');
    assert(call({name: 'count', profile: 'Personal', domains: exact}).output === '3', 'exact host, source filter before shared-tab dedupe');
    assert(call({name: 'count', profile: 'Personal', domains: [{host: 'example.com', subdomains: true}]}).output === '1', 'wildcard excludes root and includes nested subdomain');
    assert(call({name: 'count', profile: 'Personal', domains: union}).output === '4', 'multiple domains use union and ignore title/path/spoof suffix');
    assert(call({name: 'links', profile: 'Personal', domains: exact}).output.split('\n').length === 3, 'same URL in two windows retained');
    assert(call({name: 'profiles', counts: true, window: 'w1', domains: exact}).output.indexOf('Work\t1') !== -1, 'profile counts use actual membership');
    assert(call({name: 'profiles', counts: true, window: 'w1', domains: exact}).output.indexOf('Personal\t2') !== -1, 'shared tab counts in each profile');
    error({name: 'count', profile: 'Missing'}, 'No matching source profile');
    var request = {name: 'move-domain', profile: 'Personal', domains: union, target: 'Work', dryRun: false, progress: true};
    error(request, 'No matching profile name in window w2');
    assert(actions.length === 0, 'all-window preflight makes zero writes');
    assert(events.some(function (e) { return e.phase === 'preflight' && e.window === 'w2' && e.completed === 0; }), 'failed later preflight visible');
    assert(!events.some(function (e) { return e.phase === 'moving' || e.phase === 'moved' || e.phase === 'done'; }), 'preflight failure emits no move or success events');
    request.dryRun = true;
    var work2 = profile('Work', []);
    windows[1] = window('w2', [work2, personal2]);
    var preview = call(request);
    var dry = preview.move;
    assert(dry.matched === 4 && dry.moved === 0 && dry.skipped === 1 && dry.failed === 0 && dry.remaining === 3, 'dry-run plan');
    assert(preview.output.indexOf('MOVE\tw1\ta\tPersonal\tWork\thttps://EXAMPLE.COM:8443/path') !== -1, 'preview identifies move and URL');
    assert(preview.output.indexOf('SKIP\tw1\tshared\tPersonal\tWork\thttp://example.com/') !== -1, 'preview identifies target membership');
    assert(preview.output.indexOf('spoof') === -1 && preview.output.indexOf('path\t') === -1, 'preview excludes title, path and suffix spoof');
    assert(actions.length === 0, 'dry-run zero actions');
    assert(events[0].phase === 'scan' && events[events.length - 1].phase === 'done', 'preview progress brackets actual work');
    assert(events.some(function (e) { return e.phase === 'profile' && e.profile === 'Personal'; }), 'profile scans visible');
    assert(!events.some(function (e) { return e.phase === 'moving' || e.phase === 'moved' || e.phase === 'failure'; }), 'dry-run has no move events');
    assert(call({name: 'move-domain', domains: [{host: 'absent.test'}], target: 'Work', dryRun: true}).output.indexOf('No matching tabs') !== -1, 'empty preview is explicit');
    request.dryRun = false;
    b.fail = true;
    var failed = call(request);
    var firstMoving = events.findIndex(function (e) { return e.phase === 'moving'; });
    assert(events.filter(function (e, i) { return i < firstMoving && e.phase === 'preflight' && e.completed === 1; }).length === 2, 'all windows pass before move starts');
    assert(events.filter(function (e) { return e.phase === 'preflight'; }).every(function (e) { return e.actions === 0; }), 'no writes during preflight');
    var movedEvents = events.filter(function (e) { return e.phase === 'moved'; });
    assert(movedEvents.length === 1 && movedEvents[0].completed === 1 && movedEvents[0].actions === 1, 'success event follows actual move');
    assert(events[events.length - 1].phase === 'failure' && events[events.length - 1].completed === 1, 'failure retains actual completed count');
    assert(failed.move.moved === 1 && failed.move.failed === 1 && failed.move.remaining === 2, 'partial failure accounting');
    assert(failed.error.indexOf('Simulated move failure') !== -1 && failed.error.indexOf('w1') !== -1 && failed.error.indexOf('b') !== -1 && failed.error.indexOf('Work') !== -1 && actions.join(',') === 'a>Work', 'failure identifies tab and stops');
    b.fail = false;
    var retry = call(request).move;
    assert(events[events.length - 1].phase === 'done' && events[events.length - 1].completed === 2, 'retry reports only this batch moves');
    assert(retry.matched === 4 && retry.moved === 2 && retry.skipped === 2 && retry.remaining === 0, 'retry skips actual target membership');
    assert(actions.join(',') === 'a>Work,b>Work,x>Work', 'moves stay within source windows');
    error({name: 'move-domain', profile: 'Personal', domains: union, target: 'work'}, 'No matching profile');
    assert(actions.length === 3, 'missing target never writes');
    windows[1] = window('w2', [work2, profile('Work', []), personal2]);
    error(request, 'Ambiguous profile name in window w2');
    assert(actions.length === 3, 'ambiguous later window never writes');
    windows = [window('w3', [profile('Personal', [tab('transport', 'Transport', 'https://example.com')]), profile('Work', [])])];
    transportFailure = true;
    error(request, 'Progress transport failure');
    assert(actions[actions.length - 1] === 'transport>Work', 'transport failure follows successful browser write');
    assert(!events.some(function (e) { return e.phase === 'failure'; }), 'transport failure not reported as browser failure');
    transportFailure = false;
    actions = [];
    var keywordOnly = tab('keyword', 'No title match', 'file:///notes/YOUTUBE guide.txt');
    var titleOnly = tab('title', 'youtube', 'https://unrelated.test/');
    var literal = tab('literal', 'Literal', 'https://other.test/?q=[x].*');
    var sourceProfile = profile('Personal', [a, b, keywordOnly, titleOnly, literal]);
    var destinationProfile = profile('Work', []);
    windows = [window('keywords', [sourceProfile, destinationProfile])];
    assert(call({name: 'count', keywords: ['youtube']}).output === '1', 'keywords match full non-HTTP URL case-insensitively, not titles');
    assert(call({name: 'count', keywords: ['[x].*']}).output === '1', 'keywords are literal, not regex or wildcard');
    assert(call({name: 'count', keywords: ['youtube', '[x].*'], domains: [{host: 'example.com'}]}).output === '4', 'multiple keywords and domains form a union');
    assert(call({name: 'links', keywords: ['youtube']}).output === 'file:///notes/YOUTUBE guide.txt', 'links share keyword selector');
    assert(call({name: 'find', args: ['guide'], keywords: ['youtube']}).tabs.length === 1, 'find intersects keywords');
    assert(call({name: 'find', args: ['no title'], keywords: ['unrelated']}).tabs.length === 0, 'title query does not bypass keyword selector');
    assert(call({name: 'profiles', counts: true, keywords: ['youtube']}).output.indexOf('Personal\t1') !== -1, 'profile counts share URL keywords');
    var keywordPlan = call({name: 'move-domain', keywords: ['youtube'], target: 'Work', dryRun: true});
    assert(keywordPlan.move.matched === 1 && keywordPlan.output.indexOf('keyword') !== -1 && actions.length === 0, 'keyword batch preview selects URL only with zero writes');
    var unrelated = profile('Unrelated', [titleOnly]);
    unrelated.tabs.id = function () { throw Error('Unselected profile read'); };
    windows = [window('filtered', [sourceProfile, unrelated])];
    assert(call({name: 'count', profile: 'Personal'}).output === '5', 'count skips unselected profile IDs');
    assert(call({name: 'tabs', profile: 'Personal'}).tabs.length === 5, 'listing skips unselected profile IDs');
    var originalIds = sourceProfile.tabs.id;
    var idReads = 0;
    sourceProfile.tabs.id = function () { idReads++; return originalIds(); };
    call({name: 'count', profile: 'Personal'});
    assert(idReads === 1, 'unfiltered count reads IDs once without property vectors');
    idReads = 0;
    call({name: 'profiles', counts: true, profile: 'Personal'});
    assert(idReads === 1, 'unfiltered profile counts read IDs once');
    sourceProfile.tabs.id = originalIds;
    var originalURLs = sourceProfile.tabs.url;
    sourceProfile.tabs.url = function () {
        sourceProfile.items.reverse();
        return originalURLs();
    };
    error({name: 'tabs', profile: 'Personal'}, 'Tabs changed');
    error({name: 'profiles', counts: true, profile: 'Personal', keywords: ['youtube']}, 'Tabs changed');
    windows = [window('reorder', [sourceProfile, destinationProfile])];
    error({name: 'move-domain', keywords: ['youtube'], target: 'Work'}, 'Tabs changed');
    assert(actions.length === 0, 'same-size reorder cannot move wrong tab');
    sourceProfile.tabs.url = originalURLs;
    var changing = tab('changing', 'Changing', 'https://example.com/');
    var other = tab('other', 'Other', 'https://example.com/');
    sourceProfile = profile('Personal', [other, changing]);
    windows = [window('changed', [sourceProfile, destinationProfile])];
    changing.url = function () { return 'https://navigated.test/'; };
    sourceProfile.tabs.url = function () { return ['https://example.com/', 'https://example.com/']; };
    error({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work'}, 'URL changed');
    assert(actions.length === 0, 'all candidates validated before first browser write');
    error({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work', dryRun: true}, 'URL changed');
    assert(actions.length === 0, 'dry-run applies same candidate checks with zero writes');
    changing.id = function () { return 'replaced'; };
    sourceProfile.tabs.id = function () { return ['other', 'changing']; };
    sourceProfile.tabs.byId = function (id) { return id === 'changing' ? changing : other; };
    error({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work'}, 'ID changed');
    assert(actions.length === 0, 'replaced candidate fails before writes');
    changing.id = function () { throw Error('Candidate deleted'); };
    error({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work'}, 'Candidate deleted');
    assert(actions.length === 0, 'deleted candidate preserves original error and prevents all writes');
    var stable = tab('stable', 'Stable', 'https://example.com/');
    var late = tab('late', 'Late', 'https://example.com/');
    var stableSource = profile('Personal', [stable]);
    var lateSource = profile('Personal', [late]);
    lateSource.tabs.url = function () {
        stable.url = function () { return 'https://navigated.test/'; };
        return ['https://example.com/'];
    };
    windows = [window('earlier', [stableSource, profile('Work', [])]), window('later', [lateSource, profile('Work', [])])];
    error({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work'}, 'URL changed');
    assert(actions.length === 0, 'later window scan invalidates earlier candidate before any write');
    var first = tab('first', 'First', 'https://example.com/');
    var second = tab('second', 'Second', 'https://example.com/');
    var firstMove = first.move;
    first.move = function (options) {
        firstMove.call(this, options);
        second.url = function () { return 'https://navigated.test/'; };
    };
    destinationProfile = profile('Work', []);
    windows = [window('partial', [profile('Personal', [first, second]), destinationProfile])];
    var partial = call({name: 'move-domain', domains: [{host: 'example.com'}], target: 'Work'});
    assert(actions.join(',') === 'first>Work', 'mid-batch navigation never moves changed URL');
    assert(partial.move.moved === 1 && partial.move.failed === 1 && partial.move.remaining === 1, 'mid-batch change returns exact partial accounting');
    assert(partial.error.indexOf('URL changed') !== -1 && partial.error.indexOf('second') !== -1, 'partial failure names changed candidate and reason');
    actions = [];
    windows = [window('membership', [profile('Work', [shared]), profile('Personal', [shared])])];
    var membershipPlan = call({name: 'move-domain', profile: 'Personal', domains: [{host: 'example.com'}], target: 'Work'});
    assert(membershipPlan.move.skipped === 1 && actions.length === 0, 'move reads other-profile membership despite source filter');
    windows = [window('keyword-move', [profile('Personal', [keywordOnly, titleOnly]), profile('Work', [])])];
    var keywordMoved = call({name: 'move-domain', keywords: ['youtube'], target: 'Work'});
    assert(keywordMoved.move.moved === 1 && actions.join(',') === 'keyword>Work', 'keyword batch moves URL matches only');
    actions = [];
    var bulk = [], alreadyThere = [];
    for (var i = 0; i < 75; i++) {
        var item = tab('bulk-' + i, 'Bulk', 'https://example.com/' + i);
        bulk.push(item);
        if (i >= 7) alreadyThere.push(item);
    }
    var bulkSource = profile('Personal', bulk), bulkTarget = profile('Work', alreadyThere);
    windows = [window('bulk', [bulkSource, bulkTarget])];
    scalarReads = 0;
    var bulkMove = call({name: 'move-domain', profile: 'Personal', domains: exact, target: 'Work'});
    assert(bulkMove.move.matched === 75 && bulkMove.move.moved === 7 && bulkMove.move.skipped === 68, '75 matches move only seven and skip 68 existing memberships');
    assert(actions.length === 7, 'skipped tabs never receive a move call');
    assert(scalarReads <= 14, 'scalar candidate reads scale with actual moves, not skipped matches');
    assert(bulkSource.reads <= 6, 'source preflight reads are grouped rather than per match');
    actions = [];
    function skippedChange(change, dryRun) {
        var skip = tab('skip-change', 'Skip', 'https://example.com/');
        var moving = tab('moving', 'Moving', 'https://example.com/');
        var origin = profile('Personal', [moving, skip]);
        var target = profile('Work', [skip]);
        origin.onRead = function (prop, ids) { if (ids && prop === 'id') change(skip, origin); };
        windows = [window('skip-change', [origin, target])];
        error({name: 'move-domain', profile: 'Personal', domains: exact, target: 'Work', dryRun: dryRun}, change === changeURL ? 'URL changed' : 'ID changed');
        assert(actions.length === 0, 'changed skipped candidate prevents every write, including preview');
    }
    function changeURL(t) { t.url = function () { return 'https://changed.test/'; }; }
    function changeID(t) { t.id = function () { return 'replacement'; }; }
    skippedChange(changeURL, false);
    skippedChange(changeURL, true);
    skippedChange(changeID, false);
    var earlier = profile('Personal', [tab('early', 'Early', 'https://example.com/')]);
    var later = profile('Personal', [tab('later', 'Later', 'https://example.com/')]);
    later.onRead = function (prop, ids) { if (ids && prop === 'url') later.items[0].url = function () { return 'https://changed.test/'; }; };
    windows = [window('earlier', [earlier, profile('Work', [])]), window('later', [later, profile('Work', [])])];
    error({name: 'move-domain', profile: 'Personal', domains: exact, target: 'Work'}, 'URL changed');
    assert(actions.length === 0, 'later window preflight failure prevents earlier window writes');
    var sameNameA = profile('Personal', [tab('same-a', 'A', 'https://example.com/a')]);
    var sameNameB = profile('Personal', [tab('same-b', 'B', 'https://example.com/b')]);
    windows = [window('same-name', [sameNameA, sameNameB, profile('Work', [])])];
    var sameNames = call({name: 'move-domain', profile: 'Personal', domains: exact, target: 'Work', dryRun: true});
    assert(sameNames.move.remaining === 2 && actions.length === 0, 'distinct source references with same name are independently validated');
    var reorder = profile('Personal', [tab('reorder-a', 'A', 'https://example.com/a'), tab('reorder-b', 'B', 'https://example.com/b')]);
    reorder.onRead = function (prop, ids) { if (ids && prop === 'url') reorder.items.reverse(); };
    windows = [window('targeted-reorder', [reorder, profile('Work', [])])];
    error({name: 'move-domain', profile: 'Personal', domains: exact, target: 'Work'}, 'Tabs changed');
    assert(actions.length === 0, 'targeted same-size reorder cannot pair wrong URL and ID');
    actions = [];
    var rootX = tab('root-x', 'X root', 'https://x.com/feed');
    var subX = tab('sub-x', 'X sub', 'https://mobile.x.com/');
    var fixed = tab('fixed', 'Fixed', 'https://example.com/fixed'); fixed.pinned = true;
    var loose = tab('loose', 'Loose', 'https://example.com/loose');
    var local = tab('local', 'Local', 'file:///notes/x.com');
    var excluded = [{host: 'x.com'}];
    var negativeSource = profile('Personal', [rootX, subX, fixed, loose, local]);
    windows = [window('negative', [negativeSource, profile('Read', [])])];
    assert(call({name: 'count', unpinned: true, excludeDomains: excluded}).output === '2', 'negative filters intersect and leave non-HTTP URLs eligible');
    assert(call({name: 'links', unpinned: true, excludeDomains: excluded}).output.split('\n').length === 2, 'links exclude pinned and excluded hosts');
    assert(call({name: 'tabs', unpinned: true, excludeDomains: excluded}).tabs.length === 2, 'tabs use negative filters');
    assert(call({name: 'find', args: ['loose'], unpinned: true, excludeDomains: excluded}).tabs.length === 1, 'find intersects negative filters');
    assert(call({name: 'domains', unpinned: true, excludeDomains: excluded}).urls.length === 2, 'domains counts receive only eligible URLs');
    assert(call({name: 'profiles', counts: true, excludeDomains: excluded}).output.indexOf('Personal\t3') !== -1, 'profile counts honor domain exclusion');
    loose.isLoading = true;
    assert(call({name: 'profiles', counts: true, unpinned: true, excludeDomains: excluded}).output.indexOf('Personal\t2') !== -1, 'profile counts intersect unpinned and excluded domains');
    assert(call({name: 'profiles', counts: true, pinned: true, domains: [{host: 'example.com'}]}).output.indexOf('Personal\t1') !== -1, 'profile counts intersect pinned and included domains');
    assert(call({name: 'profiles', counts: true, loading: true, unpinned: true, keywords: ['loose']}).output.indexOf('Personal\t1') !== -1, 'profile counts intersect loading, pin state, and keyword');
    assert(call({name: 'profiles', counts: true, pinned: true, excludeDomains: [{host: 'example.com'}]}).output.indexOf('Personal\t0') !== -1, 'profile with no state matches retains zero count');
    var sharedFixed = profile('Shared', [fixed]);
    windows = [window('negative', [negativeSource, sharedFixed, profile('Read', [])])];
    var sharedCounts = call({name: 'profiles', counts: true, pinned: true}).output;
    assert(sharedCounts.indexOf('Personal\t1') !== -1 && sharedCounts.indexOf('Shared\t1') !== -1 && sharedCounts.indexOf('Read\t0') !== -1, 'shared pinned tab counts independently in every profile');
    windows = [window('negative', [negativeSource, profile('Read', [])])];
    var sameDomain = [{host: 'example.com'}];
    assert(call({name: 'move-domain', unpinned: true, domains: sameDomain, target: 'Read', dryRun: true}).move.matched === 1, 'normalized default move excludes pinned tabs in preview');
    assert(call({name: 'move-domain', domains: sameDomain, target: 'Read', dryRun: true}).move.matched === 2, 'include-pinned selection includes both states in preview');
    assert(call({name: 'move-domain', pinned: true, domains: sameDomain, target: 'Read', dryRun: true}).move.matched === 1, 'pinned-only selection excludes loose tabs in preview');
    assert(actions.length === 0, 'all pin policy previews make no writes');
    loose.isLoading = false;
    assert(call({name: 'count', keywords: ['x.com'], excludeDomains: excluded}).output === '1', 'excluded hosts veto URL keyword matches without excluding paths');
    assert(call({name: 'count', excludeDomains: [{host: 'x.com', subdomains: true}]}).output === '4', 'wildcard exclusion keeps root');
    assert(call({name: 'count', excludeDomains: [{host: 'x.com', exact: true}]}).output === '4', 'exact exclusion keeps subdomains');
    assert(call({name: 'count', excludeDomains: [{host: 'x.com'}, {host: 'example.com'}]}).output === '1', 'excluded domains form union');
    var negativePlan = {name: 'move-domain', unpinned: true, excludeDomains: excluded, target: 'Read', dryRun: true};
    var negativePreview = call(negativePlan);
    assert(negativePreview.move.matched === 2 && negativePreview.move.moved === 0 && actions.length === 0, 'target-only preview makes no moves');
    negativePlan.dryRun = false;
    assert(call(negativePlan).move.moved === 2 && actions.join(',') === 'loose>Read,local>Read', 'apply moves only unpinned tabs outside excluded hosts');
    assert(call({name: 'move-domain', domains: sameDomain, target: 'Read'}).move.moved === 1 && actions.join(',') === 'loose>Read,local>Read,fixed>Read', 'include-pinned apply moves fixed tab and skips loose tab already in target');
    actions = [];
    var driftA = tab('drift-a', 'A', 'https://other.test/a'), driftB = tab('drift-b', 'B', 'https://other.test/b');
    var driftSource = profile('Personal', [driftA, driftB]);
    windows = [window('drift', [driftSource, profile('Read', [])])];
    driftSource.onRead = function (prop, ids) { if (ids && prop === 'ispinned') driftB.pinned = true; };
    negativePlan.dryRun = true;
    error(negativePlan, 'became pinned');
    assert(actions.length === 0, 'pin drift aborts preview without writes');
    driftB.pinned = false; negativePlan.dryRun = false;
    error(negativePlan, 'became pinned');
    assert(actions.length === 0, 'later pin drift aborts entire preflight before writes');
    driftSource.onRead = null; driftB.pinned = false;
    var originalMove = driftA.move;
    driftA.move = function (options) { originalMove.call(this, options); driftB.pinned = true; };
    var driftResult = call(negativePlan);
    assert(driftResult.move.moved === 1 && driftResult.move.failed === 1 && driftResult.move.remaining === 1 && actions.join(',') === 'drift-a>Read', 'newly pinned tab stops apply with exact partial counts');
    assert(driftResult.error.indexOf('became pinned') !== -1, 'pin drift reports actionable reason');
    return assertions + ' batch assertions passed';
}
