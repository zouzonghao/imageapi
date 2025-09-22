module dreamifly

go 1.24.0

replace dreamifly/imagehost => ./imagehost

replace dreamifly/providers => ./providers

require (
	github.com/chai2010/webp v1.4.0
	github.com/joho/godotenv v1.5.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
)

require (
	github.com/disintegration/imaging v1.6.2 // indirect
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410 // indirect
)
