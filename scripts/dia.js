// JXA uses Apple Events, not JavaScript inside a browser tab.
var HELP = [
    'Usage: dia <command> [argument] [--window <window-id>]',
    '',
    'tabs [-d|-detail]    List linked titles; -d adds full URLs in the terminal',
    '     [-s|-st|-sl]    Sort by title (default); -sl sorts URLs and requires -d',
    'count                Count the listed tabs',
    'links                Print full URLs only, one per line, in title order',
    'current              Show the front window\'s active tab',
    'find <text>          Search titles and URLs (case-insensitive, literal)',
    'focus <tab-id>       Focus exactly one matching tab',
    'open <number>       Focus a row from this shell\'s last tabs/find list',
    'profiles             List profiles with their window IDs',
    'profile <name>       Focus exactly one matching profile (exact name)',
    'help                 Show this help',
    '',
    'Short commands: t=tabs, n=count, l=links, c=current, f=find, o=open,',
    '                fo=focus, ps=profiles, p=profile, h=help',
    '',
    '--window limits the command to one window ID. Output is tab-separated.'
].join('\n');

function cell(value) {
    // Browser-controlled text must not inject terminal escapes or extra rows.
    return String(value == null ? '' : value).replace(/[\x00-\x1f\x7f-\x9f\u2028\u2029]/g, ' ');
}

function table(header, rows) {
    return [header].concat(rows).map(function (row) {
        return row.map(cell).join('\t');
    }).join('\n');
}

function run(argv) {
    var args = argv.slice();
    var cmd = args.shift() || 'help';
    if (cmd === '--help' || cmd === '-h') cmd = 'help';
    var arities = {help: 0, tabs: 0, links: 0, count: 0, current: 0, find: 1, focus: 1, profiles: 0, profile: 1, _open: 2};
    if (!Object.prototype.hasOwnProperty.call(arities, cmd)) throw Error('Unknown command: ' + cell(cmd) + '\n' + HELP);
    var windowId = null;
    var detail = false;
    var sortBy = 'title';
    // Options follow the positional argument so names such as "--window" remain valid.
    var positional = args.splice(0, arities[cmd]);
    if (cmd === 'tabs') {
        while (args.length) {
            var option = args.shift();
            if (option === '-d' || option === '-detail') detail = true;
            else if (option === '-s' || option === '-st') sortBy = 'title';
            else if (option === '-sl') sortBy = 'url';
            else if (option === '--window' && windowId === null && args.length && args[0]) windowId = args.shift();
            else throw Error(HELP);
        }
        if (sortBy === 'url' && !detail) throw Error('Link sorting (-sl) requires -d (or -detail).');
    } else if (args.length === 2 && args[0] === '--window' && args[1]) windowId = args[1];
    else if (args.length) throw Error(HELP);
    if (positional.length !== arities[cmd] || positional.some(function (s) { return !s; })) throw Error(HELP);
    if (cmd === 'help') return HELP;

    var app = Application('Dia');
    if (!app.running()) throw Error('Dia is not running. Open Dia first.');
    if (cmd === '_open') {
        try {
            app.windows.byId(positional[0]).tabs.byId(positional[1]).focus();
        } catch (error) {
            if (error.errorNumber === -1728) throw Error('Saved tab or window no longer exists. Refresh with dia tabs or dia find.');
            throw error;
        }
        return 'Focused ' + cell(positional[1]);
    }
    var windows = app.windows();
    if (windowId !== null) {
        windows = windows.filter(function (w) { return w.id() === windowId; });
        if (!windows.length) throw Error('Window not found: ' + cell(windowId));
    }
    if (cmd === 'current') {
        if (!windows.length) throw Error('No open Dia windows.');
        var front = windows[0];
        var active = front.activeTab();
        // Dia exposes activeTab properties but does not support JXA exists() here.
        return table(['WINDOW', 'TAB', 'TITLE', 'URL'], [[front.id(), active.id(), active.title(), active.url()]]);
    }

    var rows = [];
    var matches = [];
    var seen = Object.create(null);
    var query = (positional[0] || '').toLowerCase();
    windows.forEach(function (w) {
        var wid = w.id();
        w.profiles().forEach(function (p) {
            if (cmd === 'profiles' || cmd === 'profile') {
                var name = p.name();
                rows.push([wid, p.index(), name]);
                if (cmd === 'profile' && name === positional[0]) matches.push(p);
                return;
            }
            // Bulk reads avoid one Apple Event per property per tab.
            var ids = p.tabs.id();
            var listing = cmd === 'tabs' || cmd === 'links' || cmd === 'find';
            var titles = listing ? p.tabs.title() : [];
            var urls = listing ? p.tabs.url() : [];
            if (listing && (ids.length !== titles.length || ids.length !== urls.length)) {
                throw Error('Tabs changed while reading. Please retry.');
            }
            var profileName = listing ? p.name() : '';
            ids.forEach(function (id, i) {
                var key = JSON.stringify([wid, id]);
                if (seen[key]) return;
                seen[key] = true;
                if (cmd === 'focus') {
                    if (id === positional[0]) matches.push(p.tabs.byId(id));
                } else if (cmd === 'count') {
                    rows.push(id);
                } else if (cmd === 'tabs' || cmd === 'links' || (String(titles[i]) + '\n' + String(urls[i])).toLowerCase().indexOf(query) !== -1) {
                    rows.push([wid, profileName, id, titles[i], urls[i]]);
                }
            });
        });
    });
    if (cmd === 'focus' || cmd === 'profile') {
        if (!matches.length) throw Error('No matching ' + (cmd === 'focus' ? 'tab ID' : 'profile name') + ': ' + cell(positional[0]));
        if (matches.length !== 1) throw Error('Ambiguous target. Specify --window <window-id>; profile names must be unique within that window.');
        matches[0].focus();
        return 'Focused ' + cell(positional[0]);
    }
    if (cmd === 'count') return String(rows.length);
    if (cmd === 'profiles') return table(['WINDOW', 'INDEX', 'PROFILE'], rows);
    if (cmd === 'tabs' || cmd === 'links') {
        rows = rows.map(function (row, index) {
            var key = cell(row[sortBy === 'url' ? 4 : 3]).toLowerCase();
            if (sortBy === 'url') key = key.replace(/^https?:\/\//, '');
            return {row: row, key: key, index: index};
        }).sort(function (a, b) {
            return a.key < b.key ? -1 : a.key > b.key ? 1 : a.index - b.index;
        }).map(function (entry) { return entry.row; });
    }
    if (cmd === 'links') return rows.map(function (row) { return cell(row[4]); }).filter(function (url) { return url.length > 0; }).join('\n');
    return table(['WINDOW', 'PROFILE', 'TAB', 'TITLE', 'URL'], rows);
}
