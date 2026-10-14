import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { apiJSON } from './support/api';
import { readyForLedger, ensureCurrency, createCashAccount } from './support/ledger';
import { todayISO, daysFromTodayISO } from './support/dates';

test.use({ viewport: { width: 390, height: 844 } });

test('live holdings disclose manual priority, future-only and stale quotes at 390px', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const names: string[] = [];
  for (const scenario of ['priority', 'future', 'stale']) {
    const name = `Selection ${scenario} ${suffix}`;
    names.push(name);
    const instrument = await apiJSON<{id:number;commodity_id:number}>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
      commodity_code:`SEL${scenario.toUpperCase()}${suffix}`,instrument_type:'stock',display_name:name,symbol:`S${scenario}${suffix}`,
      quote_commodity_id:currencyID,trading_commodity_id:currencyID,quantity_scale:0,price_scale:2,effective_from:daysFromTodayISO(-30)
    });
    const holding = await apiJSON<{id:number}>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
      instrument_id:instrument.id,name,opened_on:daysFromTodayISO(-30),effective_from:daysFromTodayISO(-30)
    });
    await apiJSON(page, 'POST', '/api/v1/investments/transfers/external/in', csrfToken, {
      effective_on:daysFromTodayISO(-30),holding_account_id:holding.id,commodity_id:instrument.commodity_id,
      quantity_value:'2',quantity_scale:0,carried_basis_value:'10000',carried_basis_scale:2,cost_commodity_id:currencyID,
      original_acquired_on:daysFromTodayISO(-30),source_evidence:{statement:'Selection test'}
    });
    const price = (quote_type:string, adjustment_basis:string, value:number, date:string, is_manual=false) => apiJSON(page,'POST','/api/v1/pricing/prices',csrfToken,{
      base_commodity_id:instrument.commodity_id,quote_commodity_id:currencyID,quote_type,adjustment_basis,price_value:value,price_scale:2,valuation_date:date,is_manual
    });
    if (scenario === 'priority') {
      await price('manual','raw',123,todayISO(),true);
      await price('close','raw',900,todayISO());
      await price('manual','total_return',9900,todayISO(),true);
      await price('manual','raw',8800,daysFromTodayISO(1),true);
    } else await price('close','raw',123,daysFromTodayISO(scenario==='future'?1:-20));
  }
  await page.goto('/app/investments');
  await expect(page.getByRole('button',{name:new RegExp(names[0])})).toContainText('1.23');
  await expect(page.getByRole('button',{name:new RegExp(names[1])})).toContainText('Only future prices available');
  await expect(page.getByRole('button',{name:new RegExp(names[2])})).toContainText('Stale price (over 7 days)');
  await page.getByRole('button',{name:'Gains',exact:true}).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('row').filter({hasText:names[0]})).toContainText('2.46');
  const region=page.getByRole('region',{name:'Unrealized gains',exact:true});
  await region.focus();await expect(region).toBeFocused();await page.keyboard.press('ArrowRight');
  await expect.poll(()=>region.evaluate(element=>element.scrollLeft)).toBeGreaterThan(0);
  await expect(page.getByRole('row').filter({hasText:names[1]})).toContainText('Only future prices available');
  await expect(page.getByRole('row').filter({hasText:names[2]})).toContainText('Stale price (over 7 days)');
  expect((await new AxeBuilder({page}).include('main').analyze()).violations).toEqual([]);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});

test('reporting FX shares priority and explains unusable coverage while retaining source currency', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const eur = await ensureCurrency(page,csrfToken,'EUR','Euro');
  const usd = await ensureCurrency(page,csrfToken,'USD','US Dollar');
  const quote = currencyID===eur?usd:eur;
  const cash = await createCashAccount(page,csrfToken,`Selection report ${Date.now()}`,currencyID,daysFromTodayISO(-30));
  const categories=await apiJSON<{categories:Array<{id:number;code:string}>}>(page,'GET','/api/v1/categories');
  const category=categories.categories.find(c=>c.code==='expense_food_groceries')!.id;
  await apiJSON(page,'POST','/api/v1/transactions',csrfToken,{
    transaction_date:todayISO(),status:'posted',description:'Selection FX test',journal_entries:[{entry_date:todayISO(),entry_kind:'ordinary',postings:[
      {account_id:cash.id,commodity_id:currencyID,quantity_value:'-10000',quantity_scale:2},
      {account_id:category,commodity_id:currencyID,quantity_value:'10000',quantity_scale:2}
    ]}]
  });
  const price=(quote_type:string,adjustment_basis:string,value:number,date:string,is_manual=false)=>apiJSON<{id:number}>(page,'POST','/api/v1/pricing/prices',csrfToken,{
    base_commodity_id:currencyID,quote_commodity_id:quote,quote_type,adjustment_basis,price_value:value,price_scale:2,valuation_date:date,is_manual
  });
  const manual=await price('manual','raw',80,todayISO(),true);
  const provider=await price('official_fixing','not_applicable',90,todayISO());
  await price('manual','total_return',99,todayISO(),true);
  await price('manual','raw',200,daysFromTodayISO(1),true);
  const url=`/app/reports?view=spending&group_by=category&start_date=${todayISO()}&end_date=${todayISO()}&account_id=${cash.id}&include_descendants=false&reporting_currency_id=${quote}`;
  await page.goto(url);
  await expect(page.getByText('Prices use the latest eligible date', {exact:false})).toBeVisible();
  await expect(page.getByRole('table')).toContainText('80.00');
  await expect(page.getByRole('table')).toContainText('100.00');
  for(const id of [manual.id,provider.id]) await apiJSON(page,'POST',`/api/v1/pricing/prices/${id}/void`,csrfToken,{void_reason:'Withdraw selection fixture',change_reason:'Selection test'});
  await page.reload();
  await expect(page.getByText('Only future prices available',{exact:true})).toBeVisible();
  await expect(page.getByRole('table')).toContainText('100.00');
  expect((await new AxeBuilder({page}).include('main').analyze()).violations).toEqual([]);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
