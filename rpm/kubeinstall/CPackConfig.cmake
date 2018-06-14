SET(CPACK_GENERATOR "RPM")
SET(CPACK_PACKAGE_NAME "kubeinstall")
SET(CPACK_PACKAGE_VERSION "2.0")
SET(CPACK_PACKAGE_RELEASE "1.6")
SET(CPACK_PACKAGE_DESCRIPTION "kubeinstall which will be installed in /opt/")
SET(CPACK_PACKAGE_FILE_NAME
"${CPACK_PACKAGE_NAME}-${CPACK_PACKAGE_VERSION}-${CPACK_PACKAGE_RELEASE}.${CMAKE_SYSTEM_PROCESSOR}")
SET(CPACK_INSTALL_COMMANDS
#"rm -rf $ENV{PWD}/build"
#"rm -rf $ENV{PWD}/build/svc"
#"rm -rf $ENV{PWD}/build/cfg"
#"rm -rf $ENV{PWD}/build/bin"
"mkdir -p $ENV{PWD}/build/"
"mkdir -p $ENV{PWD}/build/etc/sysconfig/kubeinstall/"
"mkdir -p $ENV{PWD}/build/usr/bin/"
"mkdir -p $ENV{PWD}/build/usr/lib/systemd/system/"
"cp $ENV{PWD}/kubeinstall.cfg /$ENV{PWD}/build/etc/sysconfig/kubeinstall/"
"cp $ENV{PWD}/../../src/kubeinstall /$ENV{PWD}/build/usr/bin/"
"chmod 777 /$ENV{PWD}/build/usr/bin/kubeinstall"
"cp $ENV{PWD}/kubeinstall.service /$ENV{PWD}/build/usr/lib/systemd/system/"
)
SET(CPACK_INSTALLED_DIRECTORIES
#SET(CPACK_PACKAGE_INSTALL_DIRECTORY
"$ENV{PWD}/build;/")
