/* Local UI translations. Never translate configuration values or arbitrary DOM text. */
(function () {
  'use strict';
  var dictionary = window.OpenVPNUIZh || {};
  var storageKey = 'openvpn-ui.language';
  var saved;
  try { saved = window.localStorage.getItem(storageKey); } catch (_) { /* Private browsing. */ }
  var language = saved === 'en' || saved === 'zh-CN' ? saved :
    (/^zh\b/i.test(navigator.language || '') ? 'zh-CN' : 'en');
  var messages = new WeakMap();
  var messagePatterns = [
    [/^Success! Certificate for the name "(.*)" has been created$/, '已创建证书“$1”'],
    [/^Success! Certificate for the name "(.*)" and serial  "(.*)" has been revoked$/, '已吊销证书“$1”（序列号：$2）'],
    [/^Success! Certificate for the name "(.*)" and serial  "(.*)"  has been removed$/, '已删除证书“$1”（序列号：$2）'],
    [/^Success! Certificate for the name "(.*)"  and IP "(.*)" and Serial "(.*)" has been renewed$/, '已续期证书“$1”（IP：$2，序列号：$3）'],
    [/^Error! There is already a valid or invalid certificate for the name "(.*)"$/, '已存在名称为“$1”的有效或无效证书'],
    [/^Success! The "(.*)" has been deleted$/, '已删除“$1”'],
    [/^Success! The "(.*)" has been initialized\.$/, '已初始化“$1”'],
    [/^Success! Container "(.*)" has been restarted$/, '已重启容器“$1”'],
    [/^User with login "(.*)" is already exists!$/, '登录名“$1”已存在'],
    [/^New user with login "(.*)" created successfully\.$/, '已创建用户“$1”'],
    [/^User  "(.*)" deleted successfully\.$/, '已删除用户“$1”'],
    [/^User "(.*)" updated successfully$/, '已更新用户“$1”'],
    [/^Failed to delete user "(.*)" profile$/, '无法删除用户“$1”的资料'],
    [/^Failed to read user "(.*)" profile$/, '无法读取用户“$1”的资料'],
    [/^Failed to update user "(.*)" profile$/, '无法更新用户“$1”的资料'],
    [/^Config has been updated but OpenVPN server was NOT reloaded: ([\s\S]*)$/, '配置已更新，但 OpenVPN 服务端未重新加载：$1']
  ];

  function translate(text) {
    if (language !== 'zh-CN') return text;
    if (Object.prototype.hasOwnProperty.call(dictionary, text)) return dictionary[text];
    return text;
  }

  function translateMessage(text) {
    if (language !== 'zh-CN') return text;
    var translated = translate(text);
    if (translated !== text) return translated;
    for (var i = 0; i < messagePatterns.length; i++) {
      var match = text.match(messagePatterns[i][0]);
      if (match) return messagePatterns[i][1].replace(/\$(\d+)/g, function (_, n) { return match[Number(n)]; });
    }
    return text; // Keep diagnostics from OpenVPN, the OS, and third parties intact.
  }

  function setContent(element, text) {
    if (element.textContent !== text) element.textContent = text;
  }

  function apply() {
    document.documentElement.lang = language;
    document.querySelectorAll('[data-i18n]').forEach(function (element) {
      setContent(element, translate(element.getAttribute('data-i18n')));
    });
    ['title', 'placeholder', 'aria-label'].forEach(function (attribute) {
      document.querySelectorAll('[data-i18n-' + attribute + ']').forEach(function (element) {
        var value = translate(element.getAttribute('data-i18n-' + attribute));
        if (element.getAttribute(attribute) !== value) element.setAttribute(attribute, value);
      });
    });
    document.querySelectorAll('[data-i18n-message]').forEach(function (element) {
      var current = element.textContent;
      var record = messages.get(element);
      if (!record || current !== record.rendered) record = { source: current.trim() };
      record.rendered = translateMessage(record.source);
      messages.set(element, record);
      setContent(element, record.rendered);
    });
    document.querySelectorAll('[data-language-selector]').forEach(function (select) {
      select.value = language;
    });
  }

  function setLanguage(value) {
    if (value !== 'en' && value !== 'zh-CN') return;
    language = value;
    try { window.localStorage.setItem(storageKey, language); } catch (_) { /* Still switch in memory. */ }
    apply();
    document.dispatchEvent(new CustomEvent('openvpn-ui:language-change', { detail: { language: language } }));
  }

  window.OpenVPNUILanguage = {
    t: translate,
    setLanguage: setLanguage,
    setText: function (element, text) {
      element.setAttribute('data-i18n', text);
      setContent(element, translate(text));
    }
  };

  function start() {
    apply();
    document.addEventListener('change', function (event) {
      if (event.target.matches('[data-language-selector]')) setLanguage(event.target.value);
    });
    // Refresh only when marked UI text is changed/inserted (e.g. modal messages).
    var selector = '[data-i18n], [data-i18n-message], [data-i18n-title], [data-i18n-placeholder], [data-i18n-aria-label]';
    new MutationObserver(function (changes) {
      var relevant = changes.some(function (change) {
        var target = change.target.nodeType === 1 ? change.target : change.target.parentElement;
        if (target && target.closest('[data-i18n-message]')) return true;
        return Array.prototype.some.call(change.addedNodes, function (node) {
          return node.nodeType === 1 && (node.matches(selector) || node.querySelector(selector));
        });
      });
      if (relevant) apply();
    }).observe(document.body, { subtree: true, childList: true, characterData: true });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
}());
