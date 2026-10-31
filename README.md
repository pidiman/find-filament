# Find Filament

Jednoduchá slovenská PWA pre mobil: **FILAMENT ME → odfotiť → voliteľný názov → HEX**.

- Bez názvu určí dominantnú farbu zo stredu fotky priamo v zariadení, aj offline.
- Ťuknutím na fotku vyberieš konkrétne miesto. Na klávesnici fungujú šípky.
- S názvom pošle zmenšenú fotku, vybraný HEX a názov serveru; OpenAI Responses API s webovým vyhľadávaním sa pokúsi nájsť presný filament na stránke výrobcu.
- Výsledok odlišuje výrobcom uvedený HEX od vizuálneho odhadu a obsahuje odkaz na zdroj. Neoverený výsledok alebo chyba služby ponechá odhad z fotky.
- Kopírovanie HEX, posledných 12 výsledkov v miestnom úložisku, inštalácia na plochu a offline režim. Fotografie sa do histórie neukladajú.
- Bez knižníc a bez zostavovania. Node.js 22+ alebo Docker.

## Spustenie

```sh
cp .env.example .env
npm start
```

Otvor `http://localhost:3000`. V `.env` nastav:

```dotenv
OPENAI_API_KEY=tvoj_kluc
OPENAI_MODEL=gpt-4.1
LOOKUP_PASSWORD=tvoje_heslo
PORT=3000
HOST=0.0.0.0
```

**Bez API kľúča fungujú všetky miestne funkcie.** Online vyhľadávanie vyžaduje platný kľúč, podporovaný model a kredit na OpenAI API (účtuje sa model aj webové vyhľadávanie). Kľúč nikdy nedávaj do `public/`, aplikácie v mobile ani Gitu. `LOOKUP_PASSWORD` chráni platené online vyhľadávanie; používateľ ho zadá v nastaveniach aplikácie a ostáva iba v pamäti otvorenej stránky. Po zmene `.env` reštartuj server.

## Nasadenie na vlastný server

### Docker

Po vytvorení `.env`:

```sh
docker compose up -d --build
```

Aplikácia počúva na `127.0.0.1:3000`. Pred ňu daj HTTPS reverzný proxy server, napríklad Caddy:

```caddyfile
filament.tvoja-domena.sk {
    reverse_proxy 127.0.0.1:3000
}
```

Pre Nginx v HTTPS server bloku:

```nginx
location / {
    proxy_pass http://127.0.0.1:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 100s;
    client_max_body_size 3m;
}
```

Odporúčané nasadenie je na samostatnej doméne/subdoméne v koreňovej ceste `/`. Pre priamy Node.js proces použi správcu procesov, napríklad systemd; `npm start` načíta `.env`. Bežný čisto statický webhosting nevykoná `server.mjs`: pre kompletné online funkcie potrebuješ Node.js alebo Docker. Samotný obsah `public/` možno hostiť staticky pre farbu z fotky, históriu a PWA; online vyhľadávanie bude nedostupné.

**Na mobile použi HTTPS.** Výnimkou pre vývoj je localhost na tom istom zariadení. Cudzia LAN IP cez HTTP neposkytuje plnohodnotnú inštalovateľnú PWA. Tlačidlo používa natívny výber fotografie s `capture="environment"`; podporované mobilné prehliadače ponúknu zadný fotoaparát, iné môžu ponúknuť výber súboru. Galéria je dostupná samostatne. Inštalácia: Android menu prehliadača → Inštalovať / Pridať na plochu; iOS Safari → Zdieľať → Pridať na plochu. Offline použitie vyžaduje aspoň jedno úspešné otvorenie online.

## Presnosť a súkromie

HEX z fotografie je odhad z najčastejšieho odtieňa v malej oblasti, nie kalibrované meranie. Foť pri rozptýlenom dennom svetle a vyber miesto bez odlesku, tieňa a cievky. Priehľadné, metalické, silk, trblietavé a viacfarebné materiály nemusia mať jeden reprezentatívny HEX. Výrobcom udaný HEX má prednosť len ak ho vyhľadávanie nájde na konkrétnej stránke; model môže napriek kontrolám urobiť chybu, preto je zdroj viditeľný na overenie.

Bez názvu fotka zariadenie neopustí. S názvom a stlačením „Nájsť môj HEX“ sa do OpenAI odošle názov a JPEG zmenšený na maximálne 1200 px spolu s vybraným odtieňom. Na serveri sa fotografie nezapisujú na disk, požiadavky používajú `store: false`; na spracovanie dát poskytovateľom sa vzťahujú jeho API podmienky. História obsahuje len HEX, názov, vysvetlenie a zdroj a dá sa vymazať tlačidlom. Service worker ukladá iba súbory aplikácie; API odpovede necachuje.

Server obmedzuje veľkosť požiadavky, počet vyhľadávaní a paralelné volania. Limit je 6 požiadaviek za minútu na priamo pripojenú IP a 3 súčasné volania; za reverzným proxy môže byť limit zdieľaný všetkými používateľmi. Pri viacerých procesoch je limit samostatný pre každý proces. Pre osobnú verejne dostupnú aplikáciu nastav `LOOKUP_PASSWORD`. Databáza nie je potrebná.

## Overenie

```sh
npm test
```

Testy pokrývajú dominantnú farbu, vstupy, odmietnutie nevystopovateľného zdroja, HTTP rozhranie, ochranu súborov, heslo, chýbajúci kľúč a požiadavku na online službu so simulovanou odpoveďou. Živé volanie OpenAI nie je súčasťou testov a vyžaduje vlastný kľúč. Na reálnom mobile ešte over otvorenie fotoaparátu, inštaláciu a offline režim v cieľovom prehliadači.

Pri aktualizácii obsahu aplikácie zvýš verziu `CACHE` v `public/sw.js`. Nový service worker sa aktivuje po zavretí starých okien aplikácie.

Technická dokumentácia: [OpenAI web search](https://developers.openai.com/api/docs/guides/tools-web-search), [obrazové vstupy](https://developers.openai.com/api/docs/guides/images-vision).
