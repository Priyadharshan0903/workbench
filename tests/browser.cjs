const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert=require('node:assert/strict');
const path=require('node:path');
const fs=require('node:fs');
const output=path.resolve(__dirname,'../.runtime/previews');fs.mkdirSync(output,{recursive:true});
(async()=>{
  const browser=await chromium.launch({headless:true,...(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:{})});
  try {
    const page=await browser.newPage({viewport:{width:1440,height:1100},deviceScaleFactor:1});
    const errors=[];page.on('pageerror',error=>errors.push(error.message));
    await page.goto(process.env.WORKBENCH_URL||'http://127.0.0.1:8730/');
    await page.locator('.tool-card').first().waitFor();
    assert(await page.locator('.tool-card').count()>=5);
    await page.screenshot({path:path.join(output,'tools-desktop.png'),fullPage:true});
    await page.locator('[data-tab=skills]').click();
    await page.locator('.skill-card').first().waitFor();
    await page.locator('#skill-provider').selectOption('claude');
    assert(await page.locator('.skill-card').count()>0);
    const cards=await page.locator('.skill-card').allTextContents();
    assert(cards.every(text=>text.includes('Claude')));
    await page.screenshot({path:path.join(output,'skills-desktop.png'),fullPage:false});
    await page.locator('.skill-card').first().click();assert(await page.locator('#details').isVisible());
    await page.locator('[data-close=details]').click();
    await page.locator('[data-tab=services]').click();await page.locator('#service-list tr').first().waitFor();
    await page.screenshot({path:path.join(output,'services-desktop.png'),fullPage:true});
    await page.locator('[data-tab=tools]').click();await page.locator('#add-tool').click();
    assert(await page.locator('#editor').isVisible());await page.locator('[data-close=editor]').click();
    for(const width of [390,768]){
      await page.setViewportSize({width,height:1000});
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Page must not overflow viewport at '+width);
      await page.screenshot({path:path.join(output,'tools-'+width+'.png'),fullPage:true});
    }
    assert.deepEqual(errors,[]);
    console.log('Browser checks passed; screenshots: '+output);
  } finally {await browser.close();}
})().catch(error=>{console.error(error.message);process.exitCode=1;});
