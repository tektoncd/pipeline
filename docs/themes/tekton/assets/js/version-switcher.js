const componentVersions = JSON.parse('[{"name": "Pipelines", "tags": [{"name": "master", "displayName": "master"}], "archive": "https://github.com/tektoncd/pipeline/tags"}, {"name": "Triggers", "tags": [{"name": "master", "displayName": "master"}], "archive": "https://github.com/tektoncd/triggers/tags"}, {"name": "CLI", "tags": [{"name": "master", "displayName": "master"}], "archive": "https://github.com/tektoncd/cli/tags"}]');

function expandOrCollapseSubMenu (componentName) {
  const childRef = `${componentName}-child`;
  const childNodes = document.querySelectorAll(`[data-ref="${childRef}"]`);
  childNodes.forEach(function (childNode) {
    childNode.classList.toggle('d-none');
  });
}

function createLatestVersionNode (anchorNode, componentName, version, disabled) {
  const latestVersionNode = document.createElement('a');
  latestVersionNode.style.paddingLeft = '2em';
  latestVersionNode.style.color = 'rgb(55, 114, 255)';
  if (disabled) {
    latestVersionNode.className = 'dropdown-item disabled';
    latestVersionNode.innerHTML = `${componentName}: ${version}`;
  } else {
    latestVersionNode.className = 'dropdown-item d-none';
    latestVersionNode.setAttribute('href', `/docs/${componentName.toLowerCase()}`);
    latestVersionNode.setAttribute('data-ref', `${componentName}-child`.toLowerCase());
    latestVersionNode.innerHTML = `Latest (${version})`;
  }
  anchorNode.appendChild(latestVersionNode);
}

function createArchivedVersionsNode (anchorNode, componentName, href) {
  const archiveNode = document.createElement('a');
  archiveNode.className = 'dropdown-item d-none';
  archiveNode.style.paddingLeft = '2em';
  archiveNode.style.color = 'rgb(55, 114, 255)';
  archiveNode.setAttribute('data-ref', `${componentName}-child`.toLowerCase());
  archiveNode.innerHTML = 'Archived Versions';
  archiveNode.setAttribute('href', href);
  anchorNode.appendChild(archiveNode);
}

function createPreviousVersionsNodes (anchorNode, componentName, previousVersions, excludedVersion) {
  previousVersions.slice().forEach(function (bundle) {
    const previousVersion = bundle.displayName;
    if (excludedVersion !== null && excludedVersion === previousVersion) {
      return;
    }

    const previousVersionNode = document.createElement('a');
    if (excludedVersion !== null) {
      previousVersionNode.className = 'dropdown-item';
    } else {
      previousVersionNode.className = 'dropdown-item d-none';
    }
    previousVersionNode.style.paddingLeft = '2em';
    previousVersionNode.style.color = 'rgb(55, 114, 255)';
    previousVersionNode.setAttribute('href', `/vault/${componentName.toLowerCase()}-${previousVersion.toLowerCase()}`);
    previousVersionNode.setAttribute('data-ref', `${componentName}-child`.toLowerCase());
    previousVersionNode.innerHTML = `${previousVersion}`;
    anchorNode.appendChild(previousVersionNode);
  });
}

function createComponentNameNode (anchorNode, componentName) {
  const componentNameNode = document.createElement('a');
  componentNameNode.className = 'dropdown-item';
  componentNameNode.innerHTML = componentName;
  componentNameNode.addEventListener('click', function () { expandOrCollapseSubMenu(componentName.toLowerCase()); });
  anchorNode.appendChild(componentNameNode);
}

function setMainVersionSwitcher () {
  const versionAnnouncerNode = document.getElementById('version-announcer');
  componentVersions.slice().forEach(function (componentVersion) {
    const componentName = componentVersion.name;
    const versions = componentVersion.tags;
    const latestVersion = versions[0].displayName;
    createLatestVersionNode(versionAnnouncerNode, componentName, latestVersion, true);
  });
  const previousReleasesFormNode = document.getElementById('previous-releases-form');
  componentVersions.slice().forEach(function (componentVersion) {
    const componentName = componentVersion.name;
    createComponentNameNode(previousReleasesFormNode, componentName);

    const versions = componentVersion.tags;
    const previousVersions = versions.slice(1);
    createPreviousVersionsNodes(previousReleasesFormNode, componentName, previousVersions, null);

    const archiveHref = componentVersion.archive;
    createArchivedVersionsNode(previousReleasesFormNode, componentName, archiveHref);
  });
}

function setVaultVersionSwitcher () {
  const pathName = window.location.pathname;
  const componentVersionStr = pathName.split('/')[2];
  const componentRawName = componentVersionStr.split('-')[0];
  const versionRawName = componentVersionStr.split('-')[1];
  let componentName;
  let versionName;

  componentVersions.forEach(function (componentVersion) {
    if (componentVersion.name.toLowerCase() === componentRawName) {
      componentName = componentVersion.name;
      componentVersion.tags.forEach(function (bundle) {
        if (bundle.displayName.toLowerCase() === versionRawName) {
          versionName = bundle.displayName;
        }
      });
    }
  });

  const currentLocationAnnouncerNode = document.getElementById('current-location-announcer');
  currentLocationAnnouncerNode.innerHTML = `${componentName} ${versionName}`;

  const currentLocationAnnouncerTextNode = document.getElementById('current-location-announcer-text');
  currentLocationAnnouncerTextNode.innerHTML = `Tekton ${componentName} ${versionName}`;

  const otherVersionsFormNode = document.getElementById('other-versions-form');
  const otherComponentsFormNode = document.getElementById('other-components-form');
  componentVersions.forEach(function (componentVersion) {
    if (componentVersion.name === componentName) {
      const latestVersion = componentVersion.tags[0].displayName;
      createLatestVersionNode(otherVersionsFormNode, componentName, latestVersion);

      const previousVersions = componentVersion.tags.slice(1);
      createPreviousVersionsNodes(otherVersionsFormNode, componentName, previousVersions, versionName);

      const archiveHref = componentVersion.archive;
      createArchivedVersionsNode(otherVersionsFormNode, componentName, archiveHref);
    } else {
      createComponentNameNode(otherComponentsFormNode, componentVersion.name);

      const latestVersion = componentVersion.tags[0].displayName;
      createLatestVersionNode(otherComponentsFormNode, componentVersion.name, latestVersion, false);

      const previousVersions = componentVersion.tags.slice(1);
      createPreviousVersionsNodes(otherComponentsFormNode, componentVersion.name, previousVersions, null);

      const archiveHref = componentVersion.archive;
      createArchivedVersionsNode(otherComponentsFormNode, componentVersion.name, archiveHref);
    }
  });
}