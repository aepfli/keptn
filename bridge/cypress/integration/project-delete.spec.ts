/// <reference types="cypress" />
import DashboardPage from '../support/pageobjects/DashboardPage';
import ProjectSettingsPage from '../support/pageobjects/ProjectSettingsPage';

describe('Project delete test', () => {
  const dashboardPage = new DashboardPage();
  const projectSettingsPage = new ProjectSettingsPage();

  it('should delete project and redirect to dashboard', () => {
    dashboardPage.intercept();
    projectSettingsPage
      .interceptSettings()
      .visitSettings('sockshop')
      .clickDeleteProjectButton()
      .typeProjectNameToDelete('sockshop')
      .submitDelete();
    dashboardPage.assertIsValidPath();
  });

  it('should be possible to delete project by pressing enter after typing project name', () => {
    dashboardPage.intercept();
    projectSettingsPage
      .interceptSettings()
      .visitSettings('sockshop')
      .clickDeleteProjectButton()
      .deleteByEnter('sockshop');
    dashboardPage.assertIsValidPath();
  });
});
