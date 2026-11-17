#requires -Version 7.4
param(
    [ValidateSet('up', 'down', 'status', 'mysql')]
    [string] $Action = 'up',
    [ValidateSet('reset')]
    [string] $MySQLAction
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
$repo = Split-Path $PSScriptRoot
$local = Join-Path $PSScriptRoot '.local'
$cluster = 'ctf-exec-local'
$node = "k3d-$cluster-server-0"
$nodeImage = 'ctf-exec-local/k3s-kata:v1.34.13-4.3.1'
$kubeconfig = Join-Path $local 'kubeconfig.yaml'
$envFile = Join-Path $repo '.env'
$compose = @('compose', '--project-name', $cluster, '--profile', 'minio', '-f', (Join-Path $PSScriptRoot 'compose.yaml'), '--env-file', $envFile)
$machine = Import-PowerShellDataFile (Join-Path $PSScriptRoot 'machine.psd1')
$k3d = $machine.K3dPath
if (!$k3d) { $k3d = Join-Path $local 'tools/k3d.exe' }

function Get-ReleaseAsset($Uri, $Path, $Hash) {
    if (!(Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Force (Split-Path $Path) | Out-Null
        Write-Host "下载 $([IO.Path]::GetFileName($Path))"
        curl.exe --fail --location --silent --show-error --retry 3 --output "$Path.part" $Uri
        if ((Get-FileHash -LiteralPath "$Path.part").Hash -ne $Hash) { throw "SHA-256 校验失败：$Path.part" }
        Move-Item -LiteralPath "$Path.part" -Destination $Path
    } elseif ((Get-FileHash -LiteralPath $Path).Hash -ne $Hash) {
        throw "SHA-256 校验失败：$Path"
    }
}

function Wait-Registry {
    $PSNativeCommandUseErrorActionPreference = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(45)
    do {
        curl.exe --noproxy '*' --silent --fail --max-time 2 --output NUL 'http://127.0.0.1:15000/v2/'
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'Registry 未就绪。使用 docker logs ctf-exec-local-registry-1 查看日志。'
}

function Wait-KubeApi {
    $PSNativeCommandUseErrorActionPreference = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(90)
    do {
        $response = kubectl --kubeconfig $kubeconfig --request-timeout=3s get --raw=/readyz 2>&1
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "Kubernetes API 未就绪：$response。使用 docker logs $node 查看节点日志。"
}

function Start-ComposeServices([string[]] $Services) {
    $missing = @()
    $stopped = @()
    foreach ($service in $Services) {
        $container = docker @compose ps -aq $service
        if (!$container) {
            $missing += $service
            continue
        }
        $expectedHash = ((docker @compose config --hash $service) -split '\s+')[1]
        $containerHash = docker inspect --format '{{index .Config.Labels "com.docker.compose.config-hash"}}' $container
        if ($containerHash -ne $expectedHash) {
            throw "$service 容器与当前 Compose 配置或 .env 不一致。请核对原容器配置与数据卷后再准备中间件。"
        }
        if ((docker inspect --format '{{.State.Running}}' $container) -ne 'true') { $stopped += $service }
    }
    if ($stopped.Count) { docker @compose start @stopped }
    if ($missing.Count) { docker @compose up -d --no-deps --no-recreate @missing }
}

function Wait-ComposeServices([string[]] $Services) {
    $containers = @(docker @compose ps -aq @Services)
    if ($containers.Count -ne $Services.Count) { throw '缺少本地依赖容器。' }
    $deadline = [DateTime]::UtcNow.AddSeconds(150)
    do {
        $states = @(docker inspect --format '{{.Name}} {{.State.Health.Status}}' @containers)
        if (!($states -notmatch ' healthy$')) { return }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "依赖服务未就绪：$($states -join '，')。使用 docker compose logs 查看日志。"
}

function Wait-MySql {
    $PSNativeCommandUseErrorActionPreference = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(120)
    do {
        docker @compose exec -T mysql sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=tcp --host=127.0.0.1 --user="$MYSQL_USER" --connect-timeout=2 -Nse "SELECT 1"' 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'MySQL 未就绪。使用 docker logs ctf-exec-local-mysql-1 查看日志。'
}

if (($Action -eq 'mysql') -ne [bool] $MySQLAction) {
    throw 'MySQL 重置命令为 mysql reset。'
}

if ($Action -in 'down', 'status' -and !(Test-Path -LiteralPath $local)) {
    Write-Host '本地环境尚未初始化。'
    return
}

if (!(Test-Path -LiteralPath $envFile -PathType Leaf)) { throw "找不到仓库根目录的 .env：$envFile" }

if ((docker --context desktop-linux info --format '{{.OSType}}') -ne 'linux') {
    throw '请启动 Docker Desktop 的 Linux 容器引擎。'
}
$desktopHost = docker context inspect desktop-linux --format '{{.Endpoints.docker.Host}}'
$previousDockerHost = $env:DOCKER_HOST
$previousDockerContext = $env:DOCKER_CONTEXT
$previousMinioPort = $env:LOCAL_MINIO_PORT
New-Item -ItemType Directory -Force $local | Out-Null
$lock = [IO.File]::Open((Join-Path $local 'dev.lock'), 'OpenOrCreate', 'ReadWrite', 'None')
try {
    $env:DOCKER_HOST = $desktopHost
    $env:DOCKER_CONTEXT = 'desktop-linux'
    if ($Action -eq 'status') {
        docker @compose ps -a
        if (Test-Path -LiteralPath $k3d) { & $k3d cluster list $cluster }
        return
    }

    if ($Action -eq 'down') {
        if (docker ps -q --filter "label=k3d.cluster=$cluster") { & $k3d cluster stop $cluster }
        docker @compose stop --timeout 45
        return
    }

    $config = docker @compose config --format json | ConvertFrom-Json
    $mysqlEnv = $config.services.mysql.environment
    if ($mysqlEnv.MYSQL_DATABASE -ne 'ctf_exec') { throw '本地数据库 SQL 要求 DB_NAME=ctf_exec。' }
    if ($mysqlEnv.MYSQL_USER -eq 'root' -and $mysqlEnv.MYSQL_PASSWORD -cne $mysqlEnv.MYSQL_ROOT_PASSWORD) {
        throw 'DB_USER=root 时，DB_PASSWORD 和 MYSQL_ROOT_PASSWORD 必须一致。'
    }

    if ($Action -eq 'mysql') {
        $sql = @(
            'DROP DATABASE IF EXISTS ctf_exec;'
            Get-Content -Raw -LiteralPath (Join-Path $repo 'db/final.sql')
            Get-ChildItem -LiteralPath (Join-Path $repo 'db/seed') -File -Filter '*.sql' | Sort-Object Name |
                ForEach-Object { Get-Content -Raw -LiteralPath $_.FullName }
        )
        $mysqlWasRunning = docker @compose ps -q --status running mysql
        Start-ComposeServices mysql
        try {
            Wait-MySql
            if (docker ps -q --filter "name=^/$node$" --filter status=running) {
                $pods = kubectl --kubeconfig $kubeconfig get pods -A -o json | ConvertFrom-Json
                $active = $pods.items | Where-Object {
                    $_.metadata.namespace -like 'ctf-*' -and $_.metadata.name -like 'chal-*' -and
                    $_.status.phase -notin 'Succeeded', 'Failed'
                }
                if ($active) { throw '请先在平台结束运行中的题目实例。' }
            }
            $sql -join "`n" | docker @compose exec -T mysql sh -ec 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql --protocol=socket --user=root --default-character-set=utf8mb4'
            Write-Host '本地 MySQL 已重建 ctf_exec 并导入 seed。'
        } finally {
            if (!$mysqlWasRunning) { docker @compose stop --timeout 45 mysql }
        }
        return
    }

    if ($machine.K3dPath) {
        if (!(Test-Path -LiteralPath $k3d -PathType Leaf)) { throw "找不到 k3d：$k3d" }
    } else {
        Get-ReleaseAsset 'https://github.com/k3d-io/k3d/releases/download/v5.9.0/k3d-windows-amd64.exe' $k3d '49d0b9c796f5b8ba6f58a1dd5469e8b7b34bf9b6ce9078633eedb2789680a034'
    }

    $existingNode = docker ps -aq --filter "name=^/$node$"
    $runningNode = docker ps -q --filter "name=^/$node$"
    $ports = @()
    if (!$runningNode) { $ports += @(80, 16443) + @(30100..30115) }
    $inUse = @(Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in $ports })
    if ($inUse.Count) { throw "端口已被占用：$($inUse.LocalPort -join ', ')" }

    $services = @('mysql', 'redis', 'rabbitmq', 'registry')
    $minioSettings = docker @compose config --environment |
        Where-Object { $_ -match '^MINIO_(ENABLED|ENDPOINT|USE_SSL)=' } | ConvertFrom-StringData
    if ($minioSettings.MINIO_ENABLED -in '1', 't', 'true') {
        $endpoint = $null
        if (![Uri]::TryCreate("http://$($minioSettings.MINIO_ENDPOINT)", [UriKind]::Absolute, [ref] $endpoint) -or
            $endpoint.Host -notin '127.0.0.1', 'localhost' -or $endpoint.Port -lt 1 -or
            $minioSettings.MINIO_ENDPOINT -notin "127.0.0.1:$($endpoint.Port)", "localhost:$($endpoint.Port)") {
            throw 'MINIO_ENDPOINT 必须填写本机地址和端口，例如 127.0.0.1:9000。'
        }
        if ($minioSettings.MINIO_USE_SSL -in '1', 't', 'true') { throw '本地 MinIO 配置要求 MINIO_USE_SSL=false。' }
        $minioEnv = $config.services.minio.environment
        if ($minioEnv.MINIO_ROOT_USER.Length -lt 3 -or $minioEnv.MINIO_ROOT_PASSWORD.Length -lt 8) {
            throw '请在 .env 中设置 MINIO_ACCESS_KEY 和 MINIO_SECRET_KEY，长度分别至少为 3 和 8 个字符。'
        }
        $env:LOCAL_MINIO_PORT = "$($endpoint.Port)"
        $services += 'minio'
    }
    Start-ComposeServices $services

    if (!$existingNode) {
        if (!(docker image ls -q $nodeImage)) {
            $kataAsset = 'kata-static-4.3.1-amd64.tar.zst'
            Get-ReleaseAsset "https://github.com/kata-containers/kata-containers/releases/download/4.3.1/$kataAsset" (Join-Path $local $kataAsset) '1463a2b86845f4d3d8e87f2f764ec59856fc0fee3520967a79ade32762be2e97'
            docker pull alpine:3.22
            docker pull rancher/k3s:v1.34.13-k3s1
            docker build -f (Join-Path $PSScriptRoot 'Kata.Dockerfile') -t $nodeImage $PSScriptRoot
        }
        & $k3d cluster create --config (Join-Path $PSScriptRoot 'k3d.yaml')
    } elseif (!$runningNode) {
        docker start $node | Out-Null
    }
    Wait-ComposeServices $services
    Wait-Registry
    & $k3d kubeconfig get $cluster | Set-Content -LiteralPath $kubeconfig
    $kube = @('--kubeconfig', $kubeconfig)
    Wait-KubeApi
    kubectl @kube wait --for=condition=Ready --timeout=180s node --all
    kubectl @kube -n kube-system wait --for=create deployment/traefik --timeout=180s
    kubectl @kube -n kube-system rollout status deployment/traefik --timeout=180s
    docker exec $node sh -ec 'test -c /dev/kvm && test -c /dev/vhost-net'
    kubectl @kube apply -f (Join-Path $PSScriptRoot 'kata-runtime.yaml')
    kubectl @kube delete pod ctf-exec-kata-smoke --ignore-not-found --timeout=30s
    kubectl @kube apply -f (Join-Path $PSScriptRoot 'kata-smoke.yaml')
    kubectl @kube wait --for=condition=Ready pod/ctf-exec-kata-smoke --timeout=180s
    $guestKernel = kubectl @kube exec ctf-exec-kata-smoke -- uname -r
    if ($guestKernel -eq (docker exec $node uname -r)) { throw 'Kata Pod 返回了宿主 Linux 内核版本。' }
    Write-Host "Kata 内核：$guestKernel"
    kubectl @kube delete pod ctf-exec-kata-smoke

    Write-Host "本地中间件已就绪。kubeconfig：$kubeconfig"
} finally {
    $env:DOCKER_HOST = $previousDockerHost
    $env:DOCKER_CONTEXT = $previousDockerContext
    $env:LOCAL_MINIO_PORT = $previousMinioPort
    $lock.Dispose()
}
