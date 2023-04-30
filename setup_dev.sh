sudo apt-get upgrade -y
sudo apt-get update -y
sudo apt-get install git -y
sudo apt-get install pulseaudio -y
cd ~
mkdir src
cd src
wget https://dl.google.com/go/go1.20.3.linux-armv6l.tar.gz
sudo tar -C /usr/local -xzf go1.20.3.linux-armv6l.tar.gz
rm go1.20.3.linux-arm64.tar.gz
cd ~
git clone https://github.com/h4ckitt/PiRecorder.git
# setup path with:
# https://www.jeremymorgan.com/tutorials/raspberry-pi/install-go-raspberry-pi/
# use raspi-config to enable legacy camera mode