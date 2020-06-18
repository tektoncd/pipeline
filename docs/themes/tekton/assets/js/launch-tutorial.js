/**
 * This script launchs interactive tutorial on a page.
 *
 */
var $ = window.$;

function getKatacodaLnk (katacodaSrc) {
  const elems = katacodaSrc.split('/');
  if (elems && elems.length !== 2) {
    throw Error('Invalid Katacoda link.');
  }
  return `${elems[0]}/scenarios/${elems[1]}`;
}

function openModal (article) {
  const katacodaSrc = article.attr('data-katacoda-src');
  const katacodaLnk = getKatacodaLnk(katacodaSrc);
  const githubLnk = article.attr('data-github-lnk');
  const qwiklabsLnk = article.attr('data-qwiklabs-lnk');

  $('#katacoda-button').attr('href', `https://katacoda.com/${katacodaLnk}`);
  $('#github-button').attr('href', `https://github.com/${githubLnk}`);
  if (!qwiklabsLnk) {
    $('qwiklabs-button').addClass('disabled');
  } else {
    $('qwiklabs-button').attr('href', `https://qwiklabs.com/${qwiklabsLnk}`);
  }

  $('#tutorial-stage').modal({ show: true, backdrop: 'static' });

  const scenario = $('#embedded-katacoda-scenario');
  const katacodaCanvasNode = document.createElement('div');
  katacodaCanvasNode.setAttribute('id', 'katacoda-scenario');
  katacodaCanvasNode.setAttribute('data-katacoda-id', katacodaSrc);
  katacodaCanvasNode.setAttribute('data-katacoda-color', '004d7f');
  katacodaCanvasNode.setAttribute('style', 'height: 900px; padding-top: 20px;');
  const katacodaScriptNode = document.createElement('script');
  katacodaScriptNode.setAttribute('id', 'katacoda-scenario-script');
  katacodaScriptNode.setAttribute('src', '//katacoda.com/embed.js');
  scenario.append(katacodaCanvasNode, katacodaScriptNode);
}

function closeModal () {
  $('#katacoda-scenario, #katacoda-scenario-script').remove();

  $('#tutorial-stage').modal('hide');

  $('#katacoda-button').removeAttr('href');
  $('#github-button').removeAttr('href');
  $('#qwiklabs-button').removeAttr('href');
}

$('.interactive-tutorial-trigger').each(function (i, elem) {
  $(elem).click(function () {
    openModal($(elem));
  });
});

$('#close-interactive-tutorial-button').click(
  closeModal
);
