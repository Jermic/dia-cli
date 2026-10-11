ObjC.import('Foundation');

function run(argv) {
    var source = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null);
    if (!source) throw Error('Cannot read CLI source');
    var focused = [];
    function tab(id, title, url) {
        return {id: function () { return id; }, title: function () { return title; },
            url: function () { return url; },
            focus: function () { focused.push(id); }};
    }
    function profile(name, tabs) {
        function collection() { return tabs; }
        ['id', 'title', 'url'].forEach(function (prop) {
            collection[prop] = function () { return tabs.map(function (t) { return t[prop](); }); };
        });
        collection.byId = function (id) { return tabs.filter(function (t) { return t.id() === id; })[0]; };
        return {name: function () { return name; }, index: function () { return 1; }, tabs: collection,
            focus: function () { focused.push(name); }};
    }
    function window(id, profiles, active) {
        return {id: function () { return id; }, profiles: function () { return profiles; },
            activeTab: function () { return active; }};
    }
    var a = tab('a', '中文 GitHub\tTitle\n\u001b[31m', 'https://example.com/?q="x"');
    var b = tab('b', 'Second', 'https://github.com/Jermic');
    var c = tab('c', 'Literal [x].*', 'https://example.org');
    var personal = profile('Personal', [a, b]);
    var work = profile('Work "quoted"', [a, c]);
    var windows = [window('w1', [personal, work], b), window('w2', [profile('Personal', [a])], a)];
    var running = true;
    var calls = 0;
    var forbidListing = false;
    var directError = null;
    function readWindows() {
        if (forbidListing) throw Error('Must not enumerate windows for cached open');
        return windows;
    }
    readWindows.byId = function (wid) {
        if (directError) throw directError;
        if (wid !== 'saved-window') throw Error('Wrong cached window ID');
        return {tabs: {byId: function (tid) {
            if (tid !== 'saved-tab') throw Error('Wrong cached tab ID');
            return {focus: function () { focused.push('saved-tab'); }};
        }}};
    };
    var cli = new Function('Application', ObjC.unwrap(source) + '\nreturn run;')(function (name) {
        calls++;
        if (name !== 'Dia') throw Error('Unexpected application');
        return {running: function () { return running; }, windows: readWindows};
    });
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
    assert(cli(['help']).indexOf('Usage: dia') === 0, 'help');
    fails(['bad'], 'Unknown command');
    fails(['focus'], 'Usage:');
    fails(['find', ''], 'Usage:');
    fails(['tabs', 'extra'], 'Usage:');
    fails(['count', '--window'], 'Usage:');
    fails(['tabs', '-sl'], 'requires -d');
    fails(['tabs', '-d', '--window'], 'Usage:');
    fails(['tabs', '-d', '--unknown'], 'Usage:');
    assert(calls === 0, 'help and validation never access Dia');
    forbidListing = true;
    cli(['_open', 'saved-window', 'saved-tab']);
    assert(focused.pop() === 'saved-tab', 'cached open focuses IDs without enumerating windows or tabs');
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
    windows = [];
    assert(cli(['count']) === '0', 'no windows count');
    assert(cli(['links']) === '', 'no windows yields no links');
    assert(cli(['tabs']).split('\n').length === 1, 'empty list header');
    fails(['current'], 'No open');
    running = false;
    fails(['count'], 'not running');
    return assertions + ' behavior assertions passed';
}
