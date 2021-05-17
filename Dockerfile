FROM scratch
COPY akari /usr/bin/akari
CMD ["/usr/bin/akari","-c","/etc/akari/akari.json"]
