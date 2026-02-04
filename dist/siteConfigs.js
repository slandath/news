export const siteConfigs = {
    'clickondetroit.com': {
        article: 'p.article-text',
        articleWrapper: ($) => $('p.article-text')
            .map((i, el) => `<p>${$(el).text()}</p>`)
            .get()
            .join(''),
        title: 'h1.headline',
    },
    'cnbc.com': {
        article: 'div.group p',
        articleWrapper: ($) => $('div.group p')
            .map((i, el) => `<p>${$(el).text()}</p>`)
            .get()
            .join(''),
        title: 'h1.ArticleHeader-headline',
    },
    'nbcnews.com': {
        article: 'p.body-graf',
        articleWrapper: ($) => $('p.body-graf')
            .map((i, el) => `<p>${$(el).text()}</p>`)
            .get()
            .join(''),
        title: 'h1.article-hero-headline__htag',
    },
    'pcgamer.com': {
        article: 'div#article-body p',
        articleWrapper: ($) => $('div#article-body p')
            .map((i, el) => `<p>${$(el).text()}</p>`)
            .get()
            .join(''),
        title: 'h1',
    },
    'slashfilm.com': {
        article: 'div.columns-holder p',
        articleWrapper: ($) => $('div.columns-holder p')
            .map((i, el) => `<p>${$(el).text()}</p>`)
            .get()
            .join(''),
        title: 'h1.title-gallery',
    },
};
//# sourceMappingURL=siteConfigs.js.map