ObjC.import('Foundation');

function cell(value) {
    return String(value == null ? '' : value).replace(/[\x00-\x1f\x7f-\x9f\u2028\u2029]/g, ' ');
}

function table(header, rows) {
    return [header].concat(rows).map(function (row) { return row.map(cell).join('\t'); }).join('\n');
}

function savedTab(window, id) {
    var missing;
    try {
        var tab = window.tabs.byId(id);
        tab.id();
        return tab;
    } catch (error) {
        if (error.errorNumber !== -1728) throw error;
        missing = error;
    }
    // Dia's window.tabs omits tabs in inactive profiles.
    var profiles = window.profiles();
    for (var i = 0; i < profiles.length; i++) {
        try {
            var candidate = profiles[i].tabs.byId(id);
            candidate.id();
            return candidate;
        } catch (error) {
            if (error.errorNumber !== -1728) throw error;
        }
    }
    throw missing;
}

function webHost(value) {
    var url = String(value == null ? '' : value);
    if (!/^https?:\/\//i.test(url) || /[\s\x00-\x1f\x7f-\x9f]/.test(url)) return '';
    var parts = $.NSURLComponents.componentsWithString(url);
    if (!parts || !parts.URL || !ObjC.unwrap(parts.host)) return '';
    return String(ObjC.unwrap(parts.host)).toLowerCase().replace(/\.$/, '');
}

function domainMatch(url, domains) {
    if (!domains || !domains.length) return true;
    var host = webHost(url);
    if (!host) return false;
    return domains.some(function (domain) {
        return domain.subdomains ? host.length > domain.host.length + 1 && host.slice(-domain.host.length - 1) === '.' + domain.host : host === domain.host;
    });
}

function run(argv) {
    if (argv.length !== 1) throw Error('Expected one JSON command.');
    var c = JSON.parse(argv[0]);
    var result = {output: ''};
    var app = Application('Dia');
    if (!app.running()) throw Error('Dia is not running. Open Dia first.');
    if (c.name.charAt(0) === '_') {
        var savedWindow, target;
        try {
            savedWindow = app.windows.byId(c.args[0]);
            target = savedTab(savedWindow, c.args[1]);
        } catch (error) {
            if (error.errorNumber === -1728) throw Error('Saved tab or window no longer exists. Refresh with dia tabs or dia find.');
            throw error;
        }
        if (c.name === '_open') target.focus();
        else if (c.name === '_close') target.close();
        else if (c.name === '_navigate') target.url = c.args[2];
        else if (c.name === '_exec') result.output = cell(target.execute({javascript: c.args[2]}));
        else if (c.name === '_move') {
            var destinations = savedWindow.profiles().filter(function (p) { return p.name() === c.args[2]; });
            if (!destinations.length) throw Error('No matching profile name in saved window: ' + cell(c.args[2]));
            if (destinations.length !== 1) throw Error('Ambiguous profile name in saved window.');
            target.move({to: destinations[0]});
        } else throw Error('Unknown saved action.');
        if (c.name !== '_exec') result.output = ({_open: 'Focused', _close: 'Closed', _navigate: 'Navigated', _move: 'Moved'})[c.name] + ' ' + cell(c.args[1]);
        return JSON.stringify(result);
    }

    var windows = app.windows();
    if (c.window) {
        windows = windows.filter(function (w) { return String(w.id()) === c.window; });
        if (!windows.length) throw Error('Window not found: ' + cell(c.window));
    }
    if (c.name === 'windows') {
        result.output = table(['WINDOW', 'PROFILE', 'LAYOUT'], windows.map(function (w) { return [w.id(), w.activeProfile().name(), w.tabLayout()]; }));
        return JSON.stringify(result);
    }
    if (c.name === 'layout') {
        if (!windows.length) throw Error('No open Dia windows.');
        windows[0].tabLayout = c.args[0];
        result.output = 'Layout ' + c.args[0];
        return JSON.stringify(result);
    }
    if (c.name === 'current') {
        if (!windows.length) throw Error('No open Dia windows.');
        var active = windows[0].activeTab();
        result.output = table(['WINDOW', 'TAB', 'TITLE', 'URL'], [[windows[0].id(), active.id(), active.title(), active.url()]]);
        return JSON.stringify(result);
    }

    var rows = [], matches = [], seen = Object.create(null), selected = 0, plans = [];
    var query = c.name === 'find' ? c.args[0].toLowerCase() : '';
    windows.forEach(function (w) {
        var wid = String(w.id());
        var profiles = w.profiles();
        var membership = Object.create(null);
        var windowCandidates = [];
        profiles.forEach(function (p) {
            var name = p.name();
            if (c.name === 'profiles' || c.name === 'profile') {
                if (c.name === 'profile' && name === c.args[0]) matches.push(p);
                if (c.name === 'profiles') {
                    if (c.profile && name !== c.profile) return;
                    selected++;
                    var profileRow = [wid, p.index(), name];
                    if (c.counts) {
                        var unique = Object.create(null);
                        var profileIds = p.tabs.id();
                        var profileURLs = c.domains && c.domains.length ? p.tabs.url() : [];
                        if (c.domains && c.domains.length && profileURLs.length !== profileIds.length) throw Error('Tabs changed while reading. Please retry.');
                        profileIds.forEach(function (id, i) {
                            if (domainMatch(profileURLs.length ? profileURLs[i] : '', c.domains)) unique[String(id)] = true;
                        });
                        profileRow.push(Object.keys(unique).length);
                    }
                    rows.push(profileRow);
                }
                return;
            }
            var ids = p.tabs.id();
            if (c.name === 'move-domain') {
                ids.forEach(function (id) {
                    var key = String(id);
                    if (!membership[key]) membership[key] = Object.create(null);
                    membership[key][name] = true;
                });
            }
            if (c.profile && name !== c.profile) return;
            selected++;
            var listing = c.name === 'tabs' || c.name === 'links' || c.name === 'find';
            var urlsNeeded = listing || c.domains && c.domains.length || c.name === 'move-domain';
            var titles = listing ? p.tabs.title() : [];
            var urls = urlsNeeded ? p.tabs.url() : [];
            var pinned = c.pinned ? p.tabs.ispinned() : [];
            var loading = c.loading ? p.tabs.loading() : [];
            if (listing && ids.length !== titles.length || urlsNeeded && ids.length !== urls.length || c.pinned && ids.length !== pinned.length || c.loading && ids.length !== loading.length) {
                throw Error('Tabs changed while reading. Please retry.');
            }
            ids.forEach(function (id, i) {
                if (c.pinned && !pinned[i] || c.loading && !loading[i] || !domainMatch(urlsNeeded ? urls[i] : '', c.domains)) return;
                var key = JSON.stringify([wid, id]);
                if (seen[key]) return;
                if (c.name === 'find' && (String(titles[i]) + '\n' + String(urls[i])).toLowerCase().indexOf(query) === -1) return;
                seen[key] = true;
                if (c.name === 'focus') {
                    if (String(id) === c.args[0]) matches.push(p.tabs.byId(id));
                } else if (c.name === 'count') rows.push(id);
                else if (c.name === 'move-domain') windowCandidates.push({window: wid, id: id, source: name, url: urls[i], tab: p.tabs.byId(id)});
                else if (listing) rows.push([wid, name, id, titles[i], urls[i]]);
            });
        });
        if (c.name === 'move-domain' && windowCandidates.length) {
            var targets = profiles.filter(function (p) { return p.name() === c.target; });
            if (!targets.length) throw Error('No matching profile name in window ' + cell(wid) + ': ' + cell(c.target));
            if (targets.length !== 1) throw Error('Ambiguous profile name in window ' + cell(wid) + ': ' + cell(c.target));
            windowCandidates.forEach(function (entry) {
                plans.push({window: entry.window, id: entry.id, source: entry.source, url: entry.url, tab: entry.tab, target: targets[0], skip: !!membership[String(entry.id)][c.target]});
            });
        }
    });
    if (c.profile && selected === 0) throw Error('No matching source profile name: ' + cell(c.profile));
    if (c.name === 'focus' || c.name === 'profile') {
        if (!matches.length) throw Error('No matching ' + (c.name === 'focus' ? 'tab ID' : 'profile name') + ': ' + cell(c.args[0]));
        if (matches.length !== 1) throw Error('Ambiguous target. Specify --window <window-id>; profile names must be unique within that window.');
        matches[0].focus();
        result.output = 'Focused ' + cell(c.args[0]);
    } else if (c.name === 'move-domain') {
        var tally = {matched: plans.length, moved: 0, skipped: 0, failed: 0, remaining: 0};
        plans.forEach(function (plan) { if (plan.skip) tally.skipped++; });
        tally.remaining = tally.matched - tally.skipped;
        if (c.dryRun) result.output = plans.length ? table(['ACTION', 'WINDOW', 'TAB', 'SOURCE', 'TARGET', 'URL'], plans.map(function (plan) {
            return [plan.skip ? 'SKIP' : 'MOVE', plan.window, plan.id, plan.source, c.target, plan.url];
        })) : 'No matching tabs.';
        if (!c.dryRun) {
            for (var i = 0; i < plans.length; i++) {
                if (plans[i].skip) continue;
                try {
                    plans[i].tab.move({to: plans[i].target});
                    tally.moved++;
                    tally.remaining--;
                } catch (error) {
                    tally.failed = 1;
                    result.error = 'Move stopped at window ' + cell(plans[i].window) + ', tab ' + cell(plans[i].id) + ', target ' + cell(c.target) + ': ' + cell(error);
                    break;
                }
            }
        }
        result.move = tally;
    } else if (c.name === 'profiles') result.output = table(c.counts ? ['WINDOW', 'INDEX', 'PROFILE', 'COUNT'] : ['WINDOW', 'INDEX', 'PROFILE'], rows);
    else if (c.name === 'count') result.output = String(rows.length);
    else {
        if (c.name === 'tabs' || c.name === 'links') {
            rows = rows.map(function (row, i) {
                var key = cell(row[c.sort === 'url' ? 4 : 3]).toLowerCase();
                if (c.sort === 'url') key = key.replace(/^https?:\/\//, '');
                return {row: row, key: key, index: i};
            }).sort(function (a, b) { return a.key < b.key ? -1 : a.key > b.key ? 1 : a.index - b.index; }).map(function (entry) { return entry.row; });
        }
        if (c.name === 'links') result.output = rows.map(function (row) { return cell(row[4]); }).filter(function (url) { return url.length > 0; }).join('\n');
        else {
            result.output = table(['WINDOW', 'PROFILE', 'TAB', 'TITLE', 'URL'], rows);
            result.tabs = rows.map(function (row) { return {window: String(row[0]), tab: String(row[2])}; });
        }
    }
    return JSON.stringify(result);
}
