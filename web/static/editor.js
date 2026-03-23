/**
 * editor.js — WYSIWYM editor behaviour for the note editing surface.
 *
 * Responsibilities
 * ────────────────
 * 1. Title extraction  – watch the textarea and copy the first `# Heading`
 *    line into the hidden <input name="title"> before every HTMX auto-save.
 *    Also updates document.title and the breadcrumb so the page always
 *    reflects the current heading without a round-trip.
 *
 * 2. Auto-resize       – grow the textarea to fit its content so the
 *    writing surface feels like a continuous document, not a box.
 *    (Progressive enhancement on top of CSS `field-sizing: content`.)
 *
 * 3. Split-view toggle – show/hide the preview pane, update aria-pressed,
 *    and fetch the server-rendered HTML so the preview is always fresh.
 *
 * 4. Live preview      – while split mode is on, re-render the preview
 *    on every keystroke after a short debounce using
 *    POST /notes/{id}/preview (the server uses goldmark + bluemonday).
 *
 * 5. Lifecycle         – init() is called on DOMContentLoaded and on
 *    htmx:afterSwap when the target is #note-display-area so the editor
 *    wires up correctly whether the fragment is loaded on initial page
 *    load or via HTMX.
 *
 * Accessibility
 * ─────────────
 * • The textarea is a native <textarea> — no focus trap.
 * • Tab / Shift-Tab behave as expected.
 * • Split-view toggle uses aria-pressed.
 * • The preview pane has aria-live="polite" (set in HTML) so updates
 *   are announced non-intrusively to screen readers.
 * • document.title is kept in sync so browser history and tab titles
 *   are meaningful.
 *
 * Security
 * ────────
 * • innerHTML on the preview pane is populated only with HTML returned
 *   by the server, which sanitises output with bluemonday.
 * • The fetch request includes credentials: 'same-origin' so the
 *   session cookie is sent and the preview endpoint is authenticated.
 */

window.notty = window.notty || {};

window.notty.editor = (function () {
  'use strict';

  /* ── Module-level state ─────────────────────────────────── */
  var textarea       = null;
  var titleInput     = null;
  var previewPane    = null;
  var editorLayout   = null;
  var splitToggle    = null;
  var breadcrumbTitle = null;
  var isSplit        = false;
  var previewDebounce = null;

  /* ── Initialise (called on load and after HTMX swaps) ───── */
  function init() {
    textarea = document.getElementById('editor-textarea');
    if (!textarea) return; // Not on an editor page — bail out silently.

    titleInput      = document.getElementById('autosave-title');
    previewPane     = document.getElementById('editor-preview-pane');
    editorLayout    = document.getElementById('editor-layout');
    splitToggle     = document.getElementById('split-toggle');
    breadcrumbTitle = document.getElementById('note-page-title');

    // ── Auto-resize ──────────────────────────────────────────
    autoResize();
    textarea.addEventListener('input', autoResize);

    // ── Title extraction ─────────────────────────────────────
    updateTitle();
    textarea.addEventListener('input', updateTitle);

    // ── HTMX: update the hidden title field before each submit ─
    // The autosave form fires on keyup; we need the title to be
    // current at the moment the request is sent.
    var form = document.getElementById('autosave-form');
    if (form) {
      form.addEventListener('htmx:beforeRequest', updateTitle);
    }

    // ── Live preview in split mode ───────────────────────────
    textarea.addEventListener('input', function () {
      if (isSplit) schedulePreviewUpdate();
    });

    // ── Split-view toggle ────────────────────────────────────
    if (splitToggle) {
      splitToggle.addEventListener('click', toggleSplit);
    }

    // ── Focus the textarea so the user can type immediately ──
    // Small delay avoids fighting with HTMX's own focus management.
    setTimeout(function () { textarea.focus(); }, 90);
  }

  /* ── Title extraction ───────────────────────────────────── */

  /**
   * Return the text of the first ATX heading (`# …`) in `content`,
   * or null if none exists.
   */
  function extractTitle(content) {
    var lines = content.split('\n');
    for (var i = 0; i < lines.length; i++) {
      var m = /^#\s+(.+)$/.exec(lines[i]);
      if (m) return m[1].trim();
    }
    return null;
  }

  /**
   * Sync the extracted heading into the hidden title input,
   * document.title, and the breadcrumb span.
   */
  function updateTitle() {
    if (!textarea || !titleInput) return;
    var t = extractTitle(textarea.value);
    if (!t) return;

    titleInput.value = t;
    document.title   = t + ' \u2014 notty';

    if (breadcrumbTitle && breadcrumbTitle.textContent !== t) {
      breadcrumbTitle.textContent = t;
    }
  }

  /* ── Auto-resize ────────────────────────────────────────── */

  /**
   * Grow the textarea to fit its content.
   * Works alongside the CSS `field-sizing: content` declaration
   * (Chrome 123+) as a fallback for older browsers.
   */
  function autoResize() {
    if (!textarea) return;
    // Reset first so shrinking works correctly.
    textarea.style.height = 'auto';
    textarea.style.height = textarea.scrollHeight + 'px';
  }

  /* ── Split-view ─────────────────────────────────────────── */

  function toggleSplit() {
    isSplit = !isSplit;
    splitToggle.setAttribute('aria-pressed', String(isSplit));

    if (editorLayout) {
      editorLayout.classList.toggle('editor-layout--split', isSplit);
    }

    if (!previewPane) return;

    if (isSplit) {
      previewPane.removeAttribute('hidden');
      updatePreview(); // Populate immediately on open.
    } else {
      previewPane.setAttribute('hidden', '');
    }
  }

  /* ── Live preview fetch ─────────────────────────────────── */

  function schedulePreviewUpdate() {
    clearTimeout(previewDebounce);
    previewDebounce = setTimeout(updatePreview, 400);
  }

  /**
   * POST the current textarea content to /notes/{id}/preview and
   * populate the preview pane with the sanitised server-rendered HTML.
   *
   * The server endpoint requires a valid session (credentials: 'same-origin')
   * and uses the same goldmark + bluemonday pipeline as the view fragment.
   */
  function updatePreview() {
    if (!isSplit || !textarea || !previewPane) return;

    var surface = document.getElementById('editor-surface');
    if (!surface) return;
    var noteId = surface.dataset.noteId;
    if (!noteId) return;

    var body = new FormData();
    body.append('content', textarea.value);

    fetch('/notes/' + encodeURIComponent(noteId) + '/preview', {
      method:      'POST',
      body:        body,
      credentials: 'same-origin',
      headers:     { 'HX-Request': 'true' }
    })
      .then(function (resp) {
        if (!resp.ok) throw new Error('Preview fetch failed: ' + resp.status);
        return resp.text();
      })
      .then(function (html) {
        // bluemonday already sanitised on the server; inserting is safe.
        previewPane.innerHTML = html;
      })
      .catch(function () {
        // Silently ignore preview errors — the auto-save still runs.
      });
  }

  /* ── Lifecycle wiring ───────────────────────────────────── */

  document.addEventListener('DOMContentLoaded', init);

  // Re-init whenever HTMX swaps the note display area (edit ↔ view).
  document.addEventListener('htmx:afterSwap', function (ev) {
    if (ev.detail && ev.detail.target && ev.detail.target.id === 'note-display-area') {
      // Reset split state on each swap so the view fragment doesn't
      // inherit stale split state from a previous edit session.
      isSplit = false;
      init();
    }
  });

  // Public surface (currently just for debugging).
  return { init: init };
}());

/**
 * Smooth-scroll handler for sidebar Table of Contents anchor links.
 *
 * Problem: the workspace layout sets `overflow: hidden` on <body> and
 * `overflow-y: auto` on `.workspace-main`.  In some browsers, native
 * anchor navigation targets the document root scroller rather than the
 * nearest scrollable ancestor, so `#heading-id` clicks jump to the top
 * of the page rather than the heading.  This handler corrects that by
 * manually computing the scroll offset and using scrollTo({behavior:'smooth'}).
 *
 * Accessibility
 * ─────────────
 * • Focus is moved to the target heading so keyboard users and screen
 *   readers land at the right place.
 * • A temporary tabindex="-1" is added when the element is not naturally
 *   focusable (headings are not by default).
 * • The URL hash is updated so the browser back-button and link sharing
 *   continue to work as expected.
 */
(function () {
  'use strict';

  document.addEventListener('click', function (e) {
    var link = e.target.closest('.sidebar-toc a[href^="#"]');
    if (!link) return;

    var hash = link.getAttribute('href'); // e.g. "#my-heading"
    var id   = hash.slice(1);
    var target = document.getElementById(id);
    if (!target) return;

    e.preventDefault();

    // Prefer .workspace-main as the scroll container; fall back to documentElement.
    var scroller = document.querySelector('.workspace-main') || document.documentElement;

    // Compute the heading's position relative to the scroller's viewport.
    var scrollerRect = scroller.getBoundingClientRect();
    var targetRect   = target.getBoundingClientRect();
    var offsetTop    = targetRect.top - scrollerRect.top + scroller.scrollTop;

    scroller.scrollTo({ top: offsetTop, behavior: 'smooth' });

    // Move focus to the heading for keyboard and screen-reader users.
    if (!target.hasAttribute('tabindex')) {
      target.setAttribute('tabindex', '-1');
    }
    target.focus({ preventScroll: true });

    // Keep URL bar in sync so users can share/bookmark the anchor.
    history.replaceState(null, '', hash);
  });
}());
