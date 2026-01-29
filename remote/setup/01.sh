#!/bin/bash

set -eu

#=====================================================#
# VARIABLES
#=====================================================#

# see full list `$ timedatectl list-timezones`
TIMEZONE=Europe/Berlin

# set os username for the application
USERNAME=greenlight

# prompt to enter a password for the db (instead of hardcoding it)
read -p "Enter password for greenlight DB user: " DB_PASSWORD

# Force all output to presented in the en_US for the duration of this script.
# This avoids any "setting locale failed" errors while this script is running, before we have
# installed support for all locales. Do not change this setting!
export LC_ALL=en_US.UTF-8

#=====================================================#
# SCRIPT LOGIC
#=====================================================#
add-apt-repository --yes universe

# update all software packages.
apt update

# set the sytem timezone and install all locales
timedatectl set-timezone ${TIMEZONE}
apt --yes install locales-all

# add new user and give them sudo privileges
useradd --create-home --shell "/bin/bash" --groups sudo "${USERNAME}"

# force password to be set for the new user the first time they log in
passwd --delete "${USERNAME}"
chage --lastday 0 "${USERNAME}"

# copy SSH keys from the root user to the new user
rsync --archive --chown=${USERNAME}:${USERNAME} /root/.ssh /home/${USERNAME}

# configure firewall to allow SSH, HTTP and HTTPS traffic.
ufw allow 22
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# install fail2ban
apt --yes install fail2ban

# install migrate tool
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.19.1/migrate.linux-amd64.tar.gz | tar xvz
mv migrate /usr/local/bin/migrate && rm LICENSE README.md

# Install postgreSQL
apt --yes install postgresql

# setup up greenlight DB and create a user account with `DB_PASSWORD`
sudo -i -u postgres psql -c "CREATE ROLE greenlight WITH LOGIN PASSWORD '${DB_PASSWORD}' CREATEDB"
sudo -i -u postgres psql -c "CREATE DATABASE greenlight OWNER greenlight"
sudo -i -u postgres psql -d greenlight -c "CREATE EXTENSION IF NOT EXISTS citext"

echo "GREENLIGHT_DB_DSN='postgres://greenlight:${DB_PASSWORD}@localhost/greenlight?sslmode=disable'" >> /etc/environment

# Install caddy
apt install --yes debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg
chmod o+r /etc/apt/sources.list.d/caddy-stable.list
apt update
apt --yes install caddy

# upgrade all packages.
# using --force-confnew flag means that configuration files will be replaced if newer ones are available
apt --yes -o Dpkg::Options::="--force-confnew" upgrade

echo "Script complete! Rebooting..."
reboot
