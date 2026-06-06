# Windows Installer Package

Este diretório contém os arquivos para criar o instalador Windows do goto.

## Para Desenvolvedores: Como Gerar o Instalador

### Pré-requisitos

1. **Inno Setup 6+** - Baixe grátis em: https://jrsoftware.org/isdl.php
2. **goto.exe compilado** - Compile com `.\build-windows.ps1` ou `go build -tags noaudio -o goto.exe .`

### Passos para Gerar o Instalador

1. **Compile o goto.exe**
   ```powershell
   cd C:\caminho\para\goto
   .\build-windows.ps1
   ```

2. **Abra o Inno Setup Compiler**
   - Instale o Inno Setup se ainda não tiver
   - Abra o arquivo `packaging/windows/installer.iss`

3. **Compile o Instalador**
   - No Inno Setup, clique em `Build > Compile` (ou pressione F9)
   - O instalador será criado em: `dist/goto_setup_0.3.16_windows_amd64.exe`

### Ou via Linha de Comando

```powershell
# Assumindo que o Inno Setup está instalado
& "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" packaging\windows\installer.iss
```

### O que o Instalador Inclui

- ✅ goto.exe (versão lightweight sem voice)
- ✅ Documentação (README, LICENSE, BUILD_WINDOWS)
- ✅ Instalação em `C:\Program Files\goto`
- ✅ Atalho no Menu Iniciar
- ✅ Opção de atalho na Área de Trabalho
- ✅ Adiciona ao PATH automaticamente
- ✅ Opção de iniciar com Windows (pausado)
- ✅ Desinstalador completo
- ✅ Suporte para Português e Inglês

## Para Usuários Finais: Como Instalar

Baixe o `goto_<versão>_installer_windows.exe` na [página de releases](https://github.com/Espigah/goto/releases/latest)
e execute. Ou use o script `scripts/install-windows.ps1`.

## Estrutura dos Arquivos

```
packaging/windows/
├── installer.iss          # Configuração do Inno Setup
└── README.md              # Este arquivo (para desenvolvedores)
```

## Notas Técnicas

### Build sem Voice vs com Voice

O instalador padrão usa a build **lightweight** (`-tags noaudio`):
- Não requer CGO
- Binário menor (~10-15 MB vs ~100+ MB)
- Funciona perfeitamente para window focus
- Não inclui reconhecimento de voz

Se quiser criar um instalador com voice:
1. Compile com: `.\build-windows.ps1 -voice`
2. Use o mesmo `installer.iss`

### Atualizando a Versão

Para gerar um instalador de uma nova versão:
1. Atualize `#define AppVersion` em `installer.iss`
2. Atualize `const version` em `main.go`
3. Recompile goto.exe e o instalador

### Distribuição

O instalador gerado (`goto_setup_*.exe`) é standalone e pode ser:
- Hospedado no GitHub Releases
- Distribuído via download direto
- Compartilhado em qualquer meio

Tamanho esperado do instalador: ~5-8 MB (sem voice) ou ~50-100 MB (com voice)
