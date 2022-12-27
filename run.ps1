go build .

$count = $args[0]
$name = $args[1]
$rps = $args[2]
$server = $args[3]

While($count -gt 0){
  $count--

  Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -red -id $($name) -rps $($rps) -host $($server)"
  Start-Sleep -Seconds 1
  Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -green -id $($name) -rps $($rps) -host $($server)"
  Start-Sleep -Seconds 1
  Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -blue -id $($name) -rps $($rps) -host $($server)"
  Start-Sleep -Seconds 1
}

