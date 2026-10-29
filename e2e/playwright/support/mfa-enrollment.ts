import { expect, type Locator, type Page } from '@playwright/test';
import { currentStep, totpCode } from './totp';

/** Activate through the real UI and return the step actually consumed. */
export async function activateBrowserMFA(
  page: Page,
  secret: string,
  controls: { code: Locator; submit: Locator },
  {
    reserveCurrentStep = false,
    // Only the fixture's samples are injectable; the app keeps its real clock.
    stepNow = currentStep
  }: { reserveCurrentStep?: boolean; stepNow?: () => number } = {}
): Promise<number> {
  let step = stepNow();
  for (let attempt = 0; attempt < 3; attempt++) {
    const activationStep = step - (reserveCurrentStep ? 1 : 0);
    await controls.code.fill(totpCode(secret, activationStep));
    const [response] = await Promise.all([
      page.waitForResponse(response =>
        new URL(response.url()).pathname === '/api/v1/auth/mfa/totp/activate' &&
        response.request().method() === 'POST'
      ),
      controls.submit.click()
    ]);
    if (response.status() === 200) return activationStep;

    // A current-step caller needs no retry. Previous-step callers refresh only
    // a proven boundary refusal, at most twice, following the #376 fixture.
    if (!reserveCurrentStep || attempt === 2) {
      expect(response.status(), 'MFA activation must succeed within the fixture attempt limit').toBe(200);
    }
    expect(response.status(), 'only an invalid-code refusal may refresh the fixture').toBe(401);
    expect(await response.json()).toEqual({
      error: { code: 'UNAUTHENTICATED', message: 'invalid authentication code' }
    });
    const freshStep = stepNow();
    expect(freshStep, 'MFA fixture may refresh only after observed step advancement').toBeGreaterThan(step);
    step = freshStep;
  }
  throw new Error('MFA activation exhausted its fixture attempts');
}
