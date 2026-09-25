# 🏐 Bot do Vôlei de Hoje

Bot de WhatsApp em Go pra confirmar presença no vôlei. Cada grupo tem uma
votação por dia: virou o dia, a contagem recomeça do zero. A lista mostra
todo mundo do grupo, com ✅ em quem vai, ❌ em quem não vai, 🤔 em quem está na
dúvida e ▫️ em quem ainda não respondeu:

```
🏐 Volei — Hoje Dia 11/09

✅ Ana
❌ Bruno
✅ Carla
🤔 Diego
▫️ Elisa

📊 Resumo
✅ Confirmados: 2
🤔 Talvez: 1
❌ Não vão: 1
▫️ Sem resposta: 1

Comente !eu pra confirmar presença no vôlei de hoje, !talvez se estiver na dúvida ou !nao se não for. Dá pra trocar quantas vezes quiser.
```

Mesmo mecanismo do `copa-volei-bot`: conecta como **aparelho conectado** usando
`whatsmeow`.

## Comandos

| Comando | O que faz |
|---|---|
| `!volei` | mostra a votação de hoje (cria se ainda não existe) |
| `!volei 17/09` | monta a lista pro dia 17/09 (título `Volei — Dia 17/09`) |
| `!volei churrasco 17/09` | lista com outro nome: `churrasco — Dia 17/09` |
| `!volei ajuda` | mostra todos os comandos com explicação (também `!volei comandos`) |
| `!volei zerar` | limpa os confirmados do dia |
| `!eu` | confirma presença |
| `!nao` | marca que não vai (❌) |
| `!talvez` | marca que está na dúvida (🤔) |
| `!eu Andressa` / `!nao Andressa Rosa` / `!talvez Andressa` | marca outra pessoa |
| `!temquepagar` / `!temquepagar 82,20` | liga a cobrança na lista atual (e define o valor por pessoa) |
| `!temquepagar 82,20 chave "Nome"` | idem, com a chave Pix e o nome de quem recebe |
| `!paguei` / `!paguei Andressa` | marca que pagou (💰) |
| `!naopaguei` / `!naopaguei Andressa` | desfaz o pagamento (volta pra 💸) |
| `!abortarmissao` | cancela o vôlei de hoje e zera a votação |
| `!add Jose` / `!add "Jose Maria"` | adiciona um nome que não está no grupo, ou põe de volta quem foi removido |
| `!remove Jose` / `!remove "Jose Maria"` | tira alguém da lista de vez (ou apaga um nome adicionado com `!add`) |
| `!volei config` | mostra quem está fora da lista e quem foi incluído à mão |

`!eu`, `!nao` e `!talvez` trocam a resposta quantas vezes a pessoa quiser — vale sempre a
última.

Pra marcar outra pessoa, vale qualquer pedaço do nome dela como aparece na
lista (`!nao Andressa`, `!nao rosa`), sem ligar pra maiúscula nem acento. Se o
pedaço bate com mais de uma pessoa, o bot mostra quem achou numerado e não marca
ninguém — aí é só mandar mais do nome ou o número no fim (`!eu jose 2`), que
vale até pra duas pessoas com o nome igualzinho. Um nome que bate inteiro ganha
dos que só contêm o pedaço. No `!add`/`!remove`, o número solto vai junto com o
nome de antes: `!remove jose 2`.

`!abortarmissao` (ou `!abortarmissão`) manda o aviso de missão abortada e limpa
os confirmados do dia — um `!volei` depois disso começa do zero.

A data do título é o dia de hoje no fuso de `VOLEI_TZ`. Qualquer comando depois
da virada do dia começa uma votação nova, sem ninguém confirmado — a do dia
anterior não volta.

Com `!volei 17/09` (ou `!volei churrasco 17/09`) a lista passa a ser pra esse
dia e continua valendo até ele acabar — `!eu`, `!nao`, `!talvez` e `!volei` seguem nela, e
no próprio dia o título ganha o "Hoje". A data é sempre a próxima vez que esse
dia chega (em dezembro, `03/01` é janeiro do ano seguinte); data que já passou
é recusada. Mudar pra outra data começa a lista do zero; mandar a mesma data
com outro nome só troca o nome e mantém quem já respondeu. `!abortarmissao`
cancela o evento marcado e volta pro vôlei de hoje.

## Cobrança

`!temquepagar` liga a cobrança só na lista atual — toda lista nova (dia novo,
`!volei 17/09`, `!abortarmissao`) começa sem cobrança. Com valor
(`!temquepagar 82,20`, também vale `82.20`, `R$ 82,20` ou `82`), aparece
`Valor por pessoa 82,20 R$` embaixo do título; mandar de novo troca o valor.
Depois do valor dá pra mandar a chave Pix e o nome de quem recebe (entre aspas
ou não): `!temquepagar 82,20 fulano@email.com "Fulano de Tal"` mostra
`Pix fulano@email.com (Fulano de Tal)` embaixo do valor. Também dá pra mandar
só o Pix (`!temquepagar fulano@email.com "Fulano de Tal"`), e o valor fica
como estava.

Com a cobrança ligada, cada pessoa com ✅ ganha uma linha embaixo do nome:

```
✅ Ana
  💸 falta pagar
✅ Diego
  💰 pago
```

O resumo no fim da lista ganha também `💰 Pagaram` e `💸 Faltam pagar`.

`!paguei` marca quem mandou (ou `!paguei Fulano` marca outra pessoa); quem paga
sem ter confirmado vira ✅. Quem pagou e depois desistiu continua com o 💰.
`!naopaguei` (ou `!naopaguei Fulano`) desfaz o pagamento marcado por engano,
sem mexer na resposta da pessoa. Dá pra ligar a cobrança sem valor e mandar
`!temquepagar 82,20` depois — o valor entra e quem já pagou continua pago.

## Config do grupo

Sem aspas, cada palavra é um nome: `!add Jose Pedro` adiciona duas pessoas.
Nome composto vai entre aspas: `!add "Jose Maria"`. Dá pra misturar:
`!add Pedro "Ana Clara"`.

`!add` e `!remove` ficam guardados por grupo e valem em todos
os dias seguintes — a votação zera na virada do dia, a config não. Quem foi
tirado some da lista (e da votação do dia), mas se mandar `!eu` ele mesmo
volta a aparecer naquele dia. Os nomes adicionados com `!add` aparecem com ▫️ como
qualquer um e dá pra marcar com `!eu Fulano` / `!nao Fulano` / `!talvez Fulano`. Qualquer pessoa
do grupo pode mexer na config.

## Lista sem poluir o grupo

Toda vez que a lista (ou o aviso de missão abortada) é mandada de novo, todas as
mensagens anteriores do dia são editadas pra só `⬇️ Lista atualizada do Volei
mais abaixo — #5 às 15:24`, citando a mais nova — tocar na citação leva direto até ela. O
número e a hora mudam a cada lista, senão o WhatsApp ignora a reedição e a
citação antiga fica. O WhatsApp só deixa editar mensagens com menos de 15
minutos, então o bot só reedita as de até 14 minutos; as mais velhas ficam como
estavam.

## Rodar

```bash
go build -buildvcs=false -o volei-hoje-bot .
./volei-hoje-bot
```

Na primeira execução aparece um QR no terminal:
**WhatsApp > Aparelhos conectados > Conectar aparelho**.

A sessão fica em `dados/sessao.db` e as votações em `dados/votacoes.json`.

Variáveis: `VOLEI_DATA` (pasta de dados, padrão `dados`), `VOLEI_LOG`
(`DEBUG`/`INFO`/`WARN`, padrão `WARN`) e `VOLEI_TZ` (fuso da data, padrão
`America/Sao_Paulo`).

## Nomes

O nome de cada um é o nome de perfil do WhatsApp. O bot só conhece o nome de
quem já mandou mensagem enquanto ele estava conectado (ou que está nos contatos
do número do bot) — quem nunca falou aparece pelo telefone até mandar a
primeira mensagem.

## Deploy na VPS

O bot roda 24/7 na VPS como serviço systemd (`volei-hoje-bot.service`, usuário
`voleihoje`, pasta `/opt/volei-hoje-bot`) com `Restart=always`, então volta
sozinho se cair ou se a máquina reiniciar.

Todo push na `main` dispara o workflow `.github/workflows/deploy.yml`, que
compila o binário para linux/amd64, manda por `scp` e reinicia o serviço. Leva
cerca de um minuto.

Secrets usados pelo workflow (em *Settings > Secrets and variables > Actions*):

| Secret | O que é |
|---|---|
| `VPS_HOST` | endereço da VPS |
| `VPS_USER` | usuário do deploy (`voleihoje`) |
| `VPS_SSH_KEY` | chave privada ed25519 só desse usuário |
| `VPS_KNOWN_HOSTS` | linha do `known_hosts` da VPS |

O usuário `voleihoje` não tem senha e só pode usar `sudo` para
`systemctl restart|status|is-active volei-hoje-bot` (`/etc/sudoers.d/voleihoje`).

A pasta `dados/` (sessão do WhatsApp e votações) vive só na VPS e nunca vai pro
git. Primeira instalação numa máquina nova: copiar `dados/sessao.db` de uma
máquina já pareada ou rodar o binário uma vez na mão pra escanear o QR.

Só uma instância pode usar a sessão por vez — rodar o bot localmente enquanto o
serviço está de pé derruba um dos dois.

Logs:

```bash
ssh usuario@vps 'journalctl -u volei-hoje-bot -f'
```
