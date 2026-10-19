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
        var child = host.length > domain.host.length + 1 && host.slice(-domain.host.length - 1) === '.' + domain.host;
        return domain.subdomains ? child : host === domain.host || !domain.exact && child;
    });
}

function urlMatch(url, domains, keywords) {
    if ((!domains || !domains.length) && (!keywords || !keywords.length)) return true;
    return !!(domains && domains.length && domainMatch(url, domains)) || !!(keywords && keywords.some(function (keyword) {
        return String(url == null ? '' : url).toLowerCase().indexOf(keyword.toLowerCase()) !== -1;
    }));
}

function readTabs(profile, properties, ids) {
    var tabs = profile.tabs;
    if (ids) {
        if (!ids.length) {
            var empty = {id: []};
            properties.forEach(function (property) { empty[property] = []; });
            return empty;
        }
        tabs = tabs.whose({_or: ids.map(function (id) { return {id: id}; })});
    }
    var snapshot = {id: tabs.id()};
    properties.forEach(function (property) { snapshot[property] = tabs[property](); });
    if (properties.length) {
        var after = tabs.id();
        if (snapshot.id.length !== after.length || snapshot.id.some(function (id, i) { return String(id) !== String(after[i]); }) || properties.some(function (property) { return snapshot[property].length !== snapshot.id.length; })) {
            throw Error('Tabs changed while reading. Please retry.');
        }
    }
    return snapshot;
}

function validatePlan(plan) {
    if (String(plan.tab.id()) !== String(plan.id)) throw Error('Tab ID changed since scan. Please retry.');
    if (String(plan.tab.url()) !== String(plan.url)) throw Error('Tab URL changed since scan. Please retry.');
}

function progress(c, phase, window, profile, completed, total) {
    if (!c.progress) return;
    var event = {phase: phase, window: cell(window), profile: cell(profile), completed: completed || 0, total: total || 0};
    var data = $(JSON.stringify({progress: event}) + '\n').dataUsingEncoding($.NSUTF8StringEncoding);
    $.NSFileHandle.fileHandleWithStandardOutput.writeData(data);
}

function dedupe(c, windows) {
    var result = {output: ''};
    var tally = {groups: 0, duplicates: 0, closed: 0, skipped: 0, failed: 0, remaining: 0};
    var groups = [], plans = [], rows = [], counted = Object.create(null), selected = 0;
    windows.forEach(function (w, wi) {
        var wid = String(w.id()), profiles = w.profiles(), membership = Object.create(null), snapshots = [];
        progress(c, 'scan', wid, '', wi, windows.length);
        profiles.forEach(function (p, pi) {
            var name = String(p.name()), key = JSON.stringify([p.index(), name]);
            var chosen = !c.profile || c.profile === name;
            progress(c, 'profile', wid, name, pi, profiles.length);
            var properties = chosen ? ['url'] : [];
            if (chosen && c.pinned) properties.push('ispinned');
            if (chosen && c.loading) properties.push('loading');
            var snapshot = readTabs(p, properties);
            snapshots.push({profile: p, name: name, key: key, snapshot: snapshot, chosen: chosen});
            snapshot.id.forEach(function (id) {
                id = String(id);
                if (!membership[id]) membership[id] = [];
                membership[id].push(key);
            });
        });
        var active = snapshots.some(function (entry) { return entry.snapshot.id.length > 0; }) ? String(w.activeTab().id()) : '';
        snapshots.forEach(function (entry) {
            if (!entry.chosen) return;
            selected++;
            var byURL = Object.create(null), snapshot = entry.snapshot;
            snapshot.id.forEach(function (id, i) {
                var url = String(snapshot.url[i] == null ? '' : snapshot.url[i]);
                if (!urlMatch(url, c.domains, c.keywords) || c.pinned && !snapshot.ispinned[i] || c.loading && !snapshot.loading[i]) return;
                if (!byURL[url]) byURL[url] = [];
                if (byURL[url].some(function (tab) { return tab.id === String(id); })) return;
                byURL[url].push({id: String(id), url: url, membership: membership[String(id)]});
            });
            var duplicateURLs = Object.keys(byURL).filter(function (url) { return byURL[url].length > 1 && webHost(url); });
            var ids = [];
            duplicateURLs.forEach(function (url) { byURL[url].forEach(function (tab) { ids.push(tab.id); }); });
            var details = readTabs(entry.profile, ['url', 'ispinned', 'loading'], ids), state = Object.create(null);
            details.id.forEach(function (id, i) { state[String(id)] = {url: String(details.url[i]), pinned: details.ispinned[i], loading: details.loading[i]}; });
            duplicateURLs.forEach(function (url) {
                var members = byURL[url];
                members.forEach(function (tab) {
                    var current = state[tab.id];
                    if (!current || current.url !== url) throw Error('Tab URL or ID changed since scan. Please retry.');
                    tab.reason = tab.membership.length > 1 ? 'shared across Profiles' : current.pinned ? 'pinned' : tab.id === active ? 'active tab' : '';
                    tab.loading = !!current.loading;
                });
                var loading = members.some(function (tab) { return tab.loading; });
                var keeper = members.filter(function (tab) { return tab.reason && !tab.loading; })[0] || members[0];
                var group = {window: w, wid: wid, profile: entry.profile, name: entry.name, key: entry.key, members: members, keeper: keeper};
                groups.push(group);
                tally.groups++;
                members.forEach(function (tab) {
                    var action = tab === keeper ? 'KEEP' : loading || tab.reason ? 'SKIP' : 'CLOSE';
                    var reason = loading ? 'group contains a loading tab' : tab.reason || (tab === keeper ? 'first copy' : 'duplicate URL');
                    rows.push([action, wid, tab.id, entry.name, url, reason]);
                    if (tab === keeper) return;
                    var physical = JSON.stringify([wid, tab.id]);
                    if (counted[physical]) return;
                    counted[physical] = true;
                    tally.duplicates++;
                    if (action === 'SKIP') tally.skipped++;
                    else { tab.close = true; plans.push({group: group, tab: tab}); }
                });
            });
        });
        progress(c, 'scan', wid, '', wi + 1, windows.length);
    });
    if (c.profile && !selected) throw Error('No matching source profile name: ' + cell(c.profile));
    tally.remaining = tally.duplicates - tally.skipped;
    var closed = Object.create(null);
    function snapshotGroups(window, selectedGroups) {
        var ids = [], bySource = Object.create(null), current = Object.create(null), sources = Object.create(null), states = Object.create(null);
        selectedGroups.forEach(function (group) {
            if (!bySource[group.key]) bySource[group.key] = [];
            group.members.forEach(function (member) {
                if (closed[JSON.stringify([group.wid, member.id])]) return;
                if (ids.indexOf(member.id) === -1) ids.push(member.id);
                if (bySource[group.key].indexOf(member.id) === -1) bySource[group.key].push(member.id);
            });
        });
        window.profiles().forEach(function (p) {
            var key = JSON.stringify([p.index(), String(p.name())]);
            var membership = readTabs(p, [], ids);
            membership.id.forEach(function (id) {
                id = String(id);
                if (!current[id]) current[id] = [];
                current[id].push(key);
            });
            if (bySource[key]) {
                sources[key] = p;
                var snapshot = readTabs(p, ['url', 'ispinned', 'loading'], bySource[key]);
                var state = states[key] = Object.create(null);
                snapshot.id.forEach(function (id, i) { state[String(id)] = {url: String(snapshot.url[i]), pinned: snapshot.ispinned[i], loading: snapshot.loading[i]}; });
            }
        });
        return {membership: current, sources: sources, states: states, active: String(window.activeTab().id())};
    }
    function validate(group, candidate, snapshot) {
        var current = snapshot.membership, source = snapshot.sources[group.key], state = snapshot.states[group.key];
        if (!source) throw Error('Profile changed since scan. Please retry.');
        var active = snapshot.active;
        group.members.forEach(function (member) {
            if (closed[JSON.stringify([group.wid, member.id])]) return;
            if (!current[member.id] || JSON.stringify(current[member.id]) !== JSON.stringify(member.membership)) throw Error('Tab membership changed since scan. Please retry.');
            var tab = state[member.id];
            if (!tab || tab.url !== member.url) throw Error('Tab URL or ID changed since scan. Please retry.');
            if (tab.loading) throw Error('Duplicate group is loading. Please retry.');
            if (member.close && (!candidate || member === candidate) && (current[member.id].length !== 1 || tab.pinned || active === member.id)) throw Error('Tab became protected since scan. Please retry.');
        });
        if (candidate) return source.tabs.byId(candidate.id);
    }
    progress(c, 'plan', '', '', tally.duplicates, tally.duplicates);
    var currentPlan;
    try {
        windows.forEach(function (window) {
            var selectedGroups = groups.filter(function (group) { return group.window === window && group.members.some(function (member) { return member.close; }); });
            if (!selectedGroups.length) return;
            var snapshot = snapshotGroups(window, selectedGroups);
            selectedGroups.forEach(function (group) { validate(group, null, snapshot); });
        });
    } catch (error) {
        tally.failed = 1;
        result.error = 'Dedupe stopped before closing tabs: ' + cell(error);
    }
    if (!result.error && !c.dryRun) {
        for (var i = 0; i < plans.length; i++) {
            currentPlan = plans[i];
            progress(c, 'closing', '', currentPlan.group.name, tally.closed, plans.length);
            try {
                validate(currentPlan.group, currentPlan.tab, snapshotGroups(currentPlan.group.window, [currentPlan.group])).close();
            } catch (error) {
                tally.failed = 1;
                result.error = 'Dedupe stopped in Profile ' + cell(currentPlan.group.name) + ': ' + cell(error);
                break;
            }
            closed[JSON.stringify([currentPlan.group.wid, currentPlan.tab.id])] = true;
            tally.closed++;
            tally.remaining--;
            progress(c, 'closed', '', currentPlan.group.name, tally.closed, plans.length);
        }
    }
    if (c.dryRun) result.output = table(['ACTION', 'WINDOW', 'TAB', 'PROFILE', 'URL', 'REASON'], rows);
    result.dedupe = tally;
    progress(c, result.error ? 'failure' : 'done', '', '', tally.closed, plans.length);
    return result;
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

    progress(c, 'scan');
    var windows = app.windows();
    if (c.window) {
        windows = windows.filter(function (w) { return String(w.id()) === c.window; });
        if (!windows.length) throw Error('Window not found: ' + cell(c.window));
    }
    if (c.name === 'dedupe') return JSON.stringify(dedupe(c, windows));
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

    var rows = [], matches = [], seen = Object.create(null), selected = 0, plans = [], planGroups = [];
    var query = c.name === 'find' ? c.args[0].toLowerCase() : '';
    var urlFiltered = !!(c.domains && c.domains.length || c.keywords && c.keywords.length);
    windows.forEach(function (w, wi) {
        var wid = String(w.id());
        progress(c, 'scan', wid, '', wi, windows.length);
        var profiles = w.profiles();
        var membership = Object.create(null);
        var windowCandidates = [];
        profiles.forEach(function (p, pi) {
            var name = p.name();
            progress(c, 'profile', wid, name, pi, profiles.length);
            if (c.name === 'profiles' || c.name === 'profile') {
                if (c.name === 'profile' && name === c.args[0]) matches.push(p);
                if (c.name === 'profiles') {
                    if (c.profile && name !== c.profile) return;
                    selected++;
                    var profileRow = [wid, p.index(), name];
                    if (c.counts) {
                        var unique = Object.create(null);
                        var profileTabs = readTabs(p, urlFiltered ? ['url'] : []);
                        profileTabs.id.forEach(function (id, i) {
                            if (urlMatch(urlFiltered ? profileTabs.url[i] : '', c.domains, c.keywords)) unique[String(id)] = true;
                        });
                        profileRow.push(Object.keys(unique).length);
                    }
                    rows.push(profileRow);
                }
                return;
            }
            if (c.name !== 'move-domain' && c.profile && name !== c.profile) return;
            var listing = c.name === 'tabs' || c.name === 'links' || c.name === 'find';
            var urlsNeeded = listing || urlFiltered || c.name === 'move-domain' || c.name === 'domains';
            var properties = [];
            var selectedProfile = !c.profile || name === c.profile;
            if (selectedProfile) {
                if (listing) properties.push('title');
                if (urlsNeeded) properties.push('url');
                if (c.pinned) properties.push('ispinned');
                if (c.loading) properties.push('loading');
            }
            var snapshot = readTabs(p, properties);
            var ids = snapshot.id;
            if (c.name === 'move-domain') {
                ids.forEach(function (id) {
                    var key = String(id);
                    if (!membership[key]) membership[key] = Object.create(null);
                    membership[key][name] = true;
                });
            }
            if (c.profile && name !== c.profile) return;
            selected++;
            var titles = snapshot.title || [];
            var urls = snapshot.url || [];
            var pinned = snapshot.ispinned || [];
            var loading = snapshot.loading || [];
            var planGroup;
            ids.forEach(function (id, i) {
                if (c.pinned && !pinned[i] || c.loading && !loading[i] || !urlMatch(urlsNeeded ? urls[i] : '', c.domains, c.keywords)) return;
                var key = JSON.stringify([wid, id]);
                if (seen[key]) return;
                if (c.name === 'find' && (String(titles[i]) + '\n' + String(urls[i])).toLowerCase().indexOf(query) === -1) return;
                seen[key] = true;
                if (c.name === 'focus') {
                    if (String(id) === c.args[0]) matches.push(p.tabs.byId(id));
                } else if (c.name === 'count') rows.push(id);
                else if (c.name === 'domains') rows.push(String(urls[i] == null ? '' : urls[i]));
                else if (c.name === 'move-domain') {
                    if (!planGroup) {
                        planGroup = {profile: p, plans: []};
                        planGroups.push(planGroup);
                    }
                    var plan = {window: wid, id: id, source: name, url: urls[i], tab: p.tabs.byId(id)};
                    planGroup.plans.push(plan);
                    windowCandidates.push(plan);
                }
                else if (listing) rows.push([wid, name, id, titles[i], urls[i]]);
            });
        });
        progress(c, 'profile', wid, '', profiles.length, profiles.length);
        if (c.name === 'move-domain' && windowCandidates.length) {
            progress(c, 'preflight', wid, c.target, 0, 1);
            var targets = profiles.filter(function (p) { return p.name() === c.target; });
            if (!targets.length) throw Error('No matching profile name in window ' + cell(wid) + ': ' + cell(c.target));
            if (targets.length !== 1) throw Error('Ambiguous profile name in window ' + cell(wid) + ': ' + cell(c.target));
            windowCandidates.forEach(function (entry) {
                entry.target = targets[0];
                entry.skip = !!membership[String(entry.id)][c.target];
                plans.push(entry);
            });
            progress(c, 'preflight', wid, c.target, 1, 1);
        }
        progress(c, 'scan', wid, '', wi + 1, windows.length);
    });
    if (c.profile && selected === 0) throw Error('No matching source profile name: ' + cell(c.profile));
    if (c.name === 'focus' || c.name === 'profile') {
        if (!matches.length) throw Error('No matching ' + (c.name === 'focus' ? 'tab ID' : 'profile name') + ': ' + cell(c.args[0]));
        if (matches.length !== 1) throw Error('Ambiguous target. Specify --window <window-id>; profile names must be unique within that window.');
        matches[0].focus();
        result.output = 'Focused ' + cell(c.args[0]);
    } else if (c.name === 'move-domain') {
        planGroups.forEach(function (group) {
            var snapshot = readTabs(group.profile, ['url'], group.plans.map(function (plan) { return plan.id; }));
            var urls = Object.create(null);
            snapshot.id.forEach(function (id, i) { urls[String(id)] = snapshot.url[i]; });
            group.plans.forEach(function (plan) {
                if (!Object.prototype.hasOwnProperty.call(urls, String(plan.id))) throw Error('Tab ID changed since scan. Please retry.');
                if (String(urls[String(plan.id)]) !== String(plan.url)) throw Error('Tab URL changed since scan. Please retry.');
            });
        });
        var tally = {matched: plans.length, moved: 0, skipped: 0, failed: 0, remaining: 0};
        plans.forEach(function (plan) { if (plan.skip) tally.skipped++; });
        tally.remaining = tally.matched - tally.skipped;
        var totalMoves = tally.remaining;
        progress(c, 'plan', '', c.target, plans.length, plans.length);
        if (c.dryRun) result.output = plans.length ? table(['ACTION', 'WINDOW', 'TAB', 'SOURCE', 'TARGET', 'URL'], plans.map(function (plan) {
            return [plan.skip ? 'SKIP' : 'MOVE', plan.window, plan.id, plan.source, c.target, plan.url];
        })) : 'No matching tabs.';
        if (!c.dryRun) {
            for (var i = 0; i < plans.length; i++) {
                if (plans[i].skip) continue;
                progress(c, 'moving', plans[i].window, c.target, tally.moved, totalMoves);
                try {
                    validatePlan(plans[i]);
                    plans[i].tab.move({to: plans[i].target});
                } catch (error) {
                    tally.failed = 1;
                    progress(c, 'failure', plans[i].window, c.target, tally.moved, totalMoves);
                    result.error = 'Move stopped at window ' + cell(plans[i].window) + ', tab ' + cell(plans[i].id) + ', target ' + cell(c.target) + ': ' + cell(error);
                    break;
                }
                tally.moved++;
                tally.remaining--;
                progress(c, 'moved', plans[i].window, c.target, tally.moved, totalMoves);
            }
        }
        result.move = tally;
        if (!result.error) progress(c, 'done', '', c.target, c.dryRun ? plans.length : tally.moved, c.dryRun ? plans.length : totalMoves);
    } else if (c.name === 'profiles') result.output = table(c.counts ? ['WINDOW', 'INDEX', 'PROFILE', 'COUNT'] : ['WINDOW', 'INDEX', 'PROFILE'], rows);
    else if (c.name === 'count') result.output = String(rows.length);
    else if (c.name === 'domains') result.urls = rows;
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
