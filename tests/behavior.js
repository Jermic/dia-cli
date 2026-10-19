ObjC.import('Foundation');

function run(argv) {
    var source = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null);
    if (!source) throw Error('Cannot read CLI source');
    var focused = [];
    var actions = [];
    var savedTarget = {id: function () { return 'saved-tab'; }, focus: function () { focused.push('saved-tab'); },
        close: function () { actions.push('close'); }, move: function (options) { actions.push(options.to.name()); },
        execute: function (options) { actions.push(options.javascript); return 'answer\n\u001b[31m'; }};
    function tab(id, title, url, pinned, loading) {
        return {ispinned: function () { return !!pinned; }, loading: function () { return !!loading; }, id: function () { return id; }, title: function () { return title; },
            url: function () { return url; },
            focus: function () { focused.push(id); }};
    }
    function profile(name, tabs) {
        function collection() { return tabs; }
        ['id', 'title', 'url', 'ispinned', 'loading'].forEach(function (prop) {
            collection[prop] = function () { return tabs.map(function (t) { return t[prop](); }); };
        });
        collection.byId = function (id) {
            var found = tabs.filter(function (t) { return t.id() === id; })[0];
            if (found) return found;
            var error = Error('Missing profile tab'); error.errorNumber = -1728; throw error;
        };
        return {name: function () { return name; }, index: function () { return 1; }, tabs: collection,
            focus: function () { focused.push(name); }};
    }
    function window(id, profiles, active) {
        return {id: function () { return id; }, profiles: function () { return profiles; },
            activeTab: function () { return active; }, activeProfile: function () { return profiles[0]; }, tabLayout: function () { return 'sidebar'; }};
    }
    var a = tab('a', '中文 GitHub\tTitle\n\u001b[31m', 'https://example.com/?q="x"');
    var b = tab('b', 'Second', 'https://github.com/Jermic', true, true);
    var c = tab('c', 'Literal [x].*', 'https://example.org', true, false);
    var personal = profile('Personal', [a, b]);
    var work = profile('Work "quoted"', [a, c]);
    var windows = [window('w1', [personal, work], b), window('w2', [profile('Personal', [a])], a)];
    var running = true;
    var calls = 0;
    var forbidListing = false;
    var directError = null;
    var savedProfiles = [work];
    var hiddenTarget = false;
    function readWindows() {
        if (forbidListing) throw Error('Must not enumerate windows for cached open');
        return windows;
    }
    readWindows.byId = function (wid) {
        if (directError) throw directError;
        if (wid !== 'saved-window') throw Error('Wrong cached window ID');
        return {profiles: function () { return savedProfiles; }, tabs: {byId: function (tid) {
            if (tid !== 'saved-tab') throw Error('Wrong cached tab ID');
            if (hiddenTarget) { var e = Error('Hidden from window tabs'); e.errorNumber = -1728; throw e; }
            return savedTarget;
        }}};
    };
    var jsonCLI = new Function('Application', ObjC.unwrap(source) + '\nreturn run;')(function (name) {
        calls++;
        if (name !== 'Dia') throw Error('Unexpected application');
        return {running: function () { return running; }, windows: readWindows};
    });
    function cli(args) {
        var count = {tabs: 0, links: 0, count: 0, current: 0, find: 1, focus: 1, profiles: 0, profile: 1, windows: 0, layout: 1,
            _open: 2, _close: 2, _move: 3, _navigate: 3, _exec: 3}[args[0]];
        if (count == null) throw Error('Unknown command');
        var c = {name: args[0], args: args.slice(1, count + 1), sort: 'title'};
        for (var i = count + 1; i < args.length; i++) {
            if (args[i] === '--window') c.window = args[++i];
            else if (args[i] === '-d' || args[i] === '-detail') c.detail = true;
            else if (args[i] === '-sl') c.sort = 'url';
            else if (args[i] === '-s' || args[i] === '-st') c.sort = 'title';
            else if (args[i] === '--pinned') c.pinned = true;
            else if (args[i] === '--loading') c.loading = true;
            else throw Error('Unexpected test option');
        }
        if (c.sort === 'url' && !c.detail) throw Error('Link sorting (-sl) requires -d');
        return JSON.parse(jsonCLI([JSON.stringify(c)])).output;
    }
    var assertions = 0;
    function assert(condition, label) {
        assertions++;
        if (!condition) throw Error('FAIL: ' + label);
    }
    function fails(args, message) {
        var before = focused.length;
        var error = '';
        try { cli(args); } catch (e) { error = String(e); }
        assert(error.indexOf(message) !== -1, 'error: ' + args.join(' '));
        assert(focused.length === before, 'failed command must not focus');
    }
    forbidListing = true;
    cli(['_open', 'saved-window', 'saved-tab']);
    assert(focused.pop() === 'saved-tab', 'cached open focuses IDs without enumerating windows or tabs');
    cli(['_close', 'saved-window', 'saved-tab']);
    assert(actions.pop() === 'close', 'close uses saved IDs');
    cli(['_navigate', 'saved-window', 'saved-tab', 'https://example.com/?q="quoted"']);
    assert(savedTarget.url === 'https://example.com/?q="quoted"', 'navigate assigns exact URL');
    cli(['_move', 'saved-window', 'saved-tab', 'Work "quoted"']);
    assert(actions.pop() === 'Work "quoted"', 'move resolves exact profile in saved window without focus');
    assert(focused.length === 0, 'move does not focus the tab or destination');
    fails(['_move', 'saved-window', 'saved-tab', 'work'], 'No matching profile');
    savedProfiles = [work, work];
    fails(['_move', 'saved-window', 'saved-tab', 'Work "quoted"'], 'Ambiguous profile');
    savedProfiles = [work];
    assert(cli(['_exec', 'saved-window', 'saved-tab', '"quoted";\n2+2']) === 'answer  [31m', 'script result controls flattened');
    assert(actions.pop() === '"quoted";\n2+2', 'script passed verbatim as data');
    var originalExecute = savedTarget.execute;
    savedTarget.execute = function () { var e = Error('Script object missing'); e.errorNumber = -1728; throw e; };
    fails(['_exec', 'saved-window', 'saved-tab', 'test()'], 'Script object missing');
    savedTarget.execute = originalExecute;
    hiddenTarget = true;
    savedProfiles = [personal, profile('Hidden', [savedTarget])];
    cli(['_close', 'saved-window', 'saved-tab']);
    assert(actions.pop() === 'close', 'saved target in hidden profile resolves by ID without listing windows');
    savedProfiles = [personal];
    fails(['_close', 'saved-window', 'saved-tab'], 'Refresh with dia tabs or dia find');
    var originalById = personal.tabs.byId;
    personal.tabs.byId = function () { var e = Error('Profile permission denied'); e.errorNumber = -1743; throw e; };
    fails(['_close', 'saved-window', 'saved-tab'], 'Profile permission denied');
    personal.tabs.byId = originalById;
    hiddenTarget = false;
    savedProfiles = [work];
    directError = new Error('Missing object');
    directError.errorNumber = -1728;
    fails(['_open', 'saved-window', 'saved-tab'], 'Refresh with dia tabs or dia find');
    directError = new Error('Automation permission denied');
    directError.errorNumber = -1743;
    fails(['_open', 'saved-window', 'saved-tab'], 'Automation permission denied');
    directError = null;
    forbidListing = false;
    assert(cli(['count']) === '4', 'all profiles/windows; dedupe within window');
    assert(cli(['count', '--window', 'w1']) === '3', 'window scope');
    assert(cli(['links', '--window', 'w1']) === 'https://example.org\nhttps://github.com/Jermic\nhttps://example.com/?q="x"', 'links only: title order, full URLs, no header or numbering');
    assert(cli(['links']).split('\n').length === 4, 'links preserve separate window occurrences');
    assert(cli(['windows']) === 'WINDOW\tPROFILE\tLAYOUT\nw1\tPersonal\tsidebar\nw2\tPersonal\tsidebar', 'windows lists active profile and layout');
    assert(cli(['windows', '--window', 'w2']).split('\n').length === 2, 'windows scope');
    cli(['layout', 'tabStrip', '--window', 'w2']);
    assert(windows[1].tabLayout === 'tabStrip' && typeof windows[0].tabLayout === 'function', 'layout changes scoped window only');
    cli(['layout', 'sidebar']);
    assert(windows[0].tabLayout === 'sidebar', 'layout defaults to front window');
    assert(cli(['tabs', '--pinned', '--window', 'w1']).split('\n').length === 3, 'pinned filter');
    assert(cli(['tabs', '--loading']).split('\n').length === 2, 'loading filter');
    assert(cli(['tabs', '--pinned', '-d', '--loading', '-sl', '--window', 'w1']).split('\n')[1].split('\t')[2] === 'b', 'filters AND with options in any order');
    var originalPinned = personal.tabs.ispinned;
    personal.tabs.ispinned = function () { return []; };
    fails(['tabs', '--pinned'], 'Tabs changed');
    personal.tabs.ispinned = originalPinned;
    var originalLoading = personal.tabs.loading;
    personal.tabs.loading = function () { return []; };
    fails(['tabs', '--loading'], 'Tabs changed');
    personal.tabs.loading = originalLoading;
    var listed = cli(['tabs']);
    assert(listed.split('\n').length === 5, 'one row per tab');
    assert(listed.indexOf('\u001b') === -1, 'terminal escape filtered');
    assert(listed.indexOf('中文 GitHub Title  [31m') !== -1, 'Unicode kept; controls flattened');
    assert(cli(['find', 'GITHUB', '--window', 'w1']).split('\n').length === 3, 'case-insensitive title and URL search');
    assert(cli(['find', '[x].*']).split('\n').length === 2, 'literal not regex');
    assert(cli(['find', 'missing']).split('\n').length === 1, 'no matches');
    assert(cli(['current']).indexOf('w1\tb\tSecond') !== -1, 'current front tab');
    assert(cli(['current', '--window', 'w2']).indexOf('w2\ta\t') !== -1, 'scoped current');
    assert(cli(['profiles']).split('\n').length === 4, 'all profiles');
    fails(['focus', 'missing'], 'No matching');
    fails(['focus', 'a'], 'Ambiguous');
    cli(['focus', 'a', '--window', 'w1']);
    assert(focused.pop() === 'a', 'focus deduplicated exact ID');
    fails(['profile', 'Personal'], 'Ambiguous');
    cli(['profile', 'Personal', '--window', 'w2']);
    assert(focused.pop() === 'Personal', 'profile window scope');
    cli(['profile', 'Work "quoted"']);
    assert(focused.pop() === 'Work "quoted"', 'quotes are data');
    fails(['profile', 'personal'], 'No matching');
    fails(['tabs', '--window', 'missing'], 'Window not found');
    work.tabs.title = function () { return []; };
    fails(['tabs'], 'Tabs changed');
    windows = [window('sort', [profile('Sort', [
        tab('z', 'zebra', 'http://a.example/B'),
        tab('a1', 'Alpha', 'https://Z.example'),
        tab('a2', 'alpha', 'https://a.example/b'),
        tab('zh', '中文', 'https://b.example'),
        tab('empty', '', '')
    ])], a)];
    function ids(args) {
        return cli(args).split('\n').slice(1).map(function (row) { return row.split('\t')[2]; }).join(',');
    }
    assert(ids(['tabs']) === 'empty,a1,a2,z,zh', 'default title sort; case-insensitive and stable');
    assert(ids(['tabs', '-s']) === 'empty,a1,a2,z,zh', 'short title flag');
    assert(ids(['tabs', '-st', '-d']) === 'empty,a1,a2,z,zh', 'explicit title flag');
    assert(ids(['tabs', '-d', '-sl']) === 'empty,z,a2,zh,a1', 'URL sort ignores scheme and case; stable ties');
    assert(ids(['tabs', '-sl', '--window', 'sort', '-detail']) === 'empty,z,a2,zh,a1', 'detail and window options in any order');
    assert(ids(['tabs', '-sl', '-st']) === 'empty,a1,a2,z,zh', 'last sort flag wins');
    fails(['tabs', '-st', '-sl'], 'requires -d');
    assert(ids(['tabs', '-st', '-sl', '-d']) === 'empty,z,a2,zh,a1', 'last URL sort flag wins');
    assert(cli(['tabs', '-d', '-sl']).indexOf('http://a.example/B') !== -1, 'sorting preserves original URLs');
    assert(cli(['links']) === 'https://Z.example\nhttps://a.example/b\nhttp://a.example/B\nhttps://b.example', 'links omit empty URLs without changing addresses');
    var reorderTabs = [tab('left', 'Left', 'https://same.test/'), tab('right', 'Right', 'https://same.test/')];
    var reorderProfile = profile('Reordering', reorderTabs);
    var readTitles = reorderProfile.tabs.title;
    reorderProfile.tabs.title = function () { reorderTabs.reverse(); return readTitles(); };
    windows = [window('reordered', [reorderProfile], reorderTabs[0])];
    fails(['tabs'], 'Tabs changed');
    var domainTabs = [tab('root', 'Root', 'https://example.com', true, true),
        tab('child', 'Child', 'https://docs.example.com/path', true, false),
        tab('duplicate-url', 'Same URL', 'https://example.com'), tab('empty-url', 'Empty', '')];
    var domainProfile = profile('Domain', domainTabs);
    domainProfile.tabs.title = function () { throw Error('Domains must not read titles'); };
    var otherDomainProfile = profile('Other', [domainTabs[0], tab('other', 'Other', 'about:blank')]);
    otherDomainProfile.tabs.title = domainProfile.tabs.title;
    windows = [window('d1', [domainProfile, otherDomainProfile], domainTabs[0]), window('d2', [profile('Domain', [domainTabs[0]])], domainTabs[0])];
    function domainURLs(flags) {
        var request = {name: 'domains'};
        Object.keys(flags || {}).forEach(function (key) { request[key] = flags[key]; });
        return JSON.parse(jsonCLI([JSON.stringify(request)])).urls;
    }
    var beforeDomainFocus = focused.length, beforeDomainActions = actions.length;
    assert(domainURLs().join('|') === 'https://example.com|https://docs.example.com/path|https://example.com||about:blank|https://example.com', 'domains dedupe window-tab pairs, retain duplicate URLs and empty URLs');
    assert(domainURLs({window: 'd1', profile: 'Domain'}).length === 4, 'domains window and profile filters intersect');
    assert(domainURLs({domains: [{host: 'example.com'}]}).length === 4, 'domains root filter includes child and separate windows');
    assert(domainURLs({domains: [{host: 'example.com', exact: true}]}).length === 3, 'domains exact filter excludes child');
    assert(domainURLs({domains: [{host: 'example.com', subdomains: true}]}).join() === 'https://docs.example.com/path', 'domains wildcard excludes root');
    assert(domainURLs({keywords: ['PATH', 'about:']}).join('|') === 'https://docs.example.com/path|about:blank', 'domains repeated keyword union is case insensitive');
    assert(domainURLs({domains: [{host: 'missing.com'}], keywords: ['about:']}).join() === 'about:blank', 'domains and keywords form union');
    assert(domainURLs({pinned: true, loading: true}).length === 2, 'domains pinned and loading intersect');
    assert(domainURLs({keywords: ['absent']}).length === 0, 'domains no matches');
    var otherDomainIDs = otherDomainProfile.tabs.id;
    otherDomainProfile.tabs.id = function () { throw Error('Unselected profile must not be read'); };
    assert(domainURLs({profile: 'Domain'}).length === 5, 'domains skip unselected profile before reading IDs');
    otherDomainProfile.tabs.id = otherDomainIDs;
    var originalDomainURLs = domainProfile.tabs.url;
    domainProfile.tabs.url = function () { domainTabs.reverse(); return originalDomainURLs(); };
    var changedDomainError = '';
    try { domainURLs(); } catch (error) { changedDomainError = String(error); }
    assert(changedDomainError.indexOf('Tabs changed') !== -1, 'domains rejects same-length reordered snapshot');
    assert(focused.length === beforeDomainFocus && actions.length === beforeDomainActions, 'domains never mutate browser');
    windows = [];
    assert(domainURLs().length === 0, 'domains no windows yields empty URLs');
    assert(cli(['count']) === '0', 'no windows count');
    assert(cli(['links']) === '', 'no windows yields no links');
    assert(cli(['tabs']).split('\n').length === 1, 'empty list header');
    fails(['current'], 'No open');
    fails(['layout', 'sidebar'], 'No open');
    assert(cli(['windows']) === 'WINDOW\tPROFILE\tLAYOUT', 'empty windows header');
    running = false;
    fails(['count'], 'not running');
    return assertions + ' behavior assertions passed';
}
