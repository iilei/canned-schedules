// The playground lived at the site's root until the docs moved in; links shared from it (?preset=, ?arg=, #doc=, …)
// still point there. On the start page, such a link goes on to playground/ with everything it carries.
(function () {
  var root = document.documentElement.dataset.content_root === "./";
  var start = /\/(index\.html)?$/.test(location.pathname);
  if (root && start && (location.search || /^#doc=/.test(location.hash))) {
    location.replace("playground/" + location.search + location.hash);
  }
})();
