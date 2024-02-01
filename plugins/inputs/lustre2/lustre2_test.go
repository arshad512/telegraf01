//go:build linux

package lustre2

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/influxdata/toml"
	"github.com/influxdata/toml/ast"
	"github.com/influxdata/telegraf/testutil"
)

/*
 * Description: Read testcases/<testname> folder and then read <input.out> file
 *
 * @t: Reference to testutil
 * @parentFolderName: Parent folder under plugins/input. (testcases)
 * @folderName: Folder holding input data of the test case
 * @inputFile: Name of file holding test input data
 *
 * Output: On success return data read from input file
 *         On Failure exit program
 */
func ReadDir(t *testing.T, parentFolderName string, folderName string,
	     inputFile string,) ([]byte) {
	testpath := filepath.Join(parentFolderName, folderName)
	_, err := os.ReadDir(testpath)
	require.NoError(t, err)
	data, err := os.ReadFile(testpath + "/" + inputFile)
	require.NoError(t, err)
	return data
}

/*
 * Description: Create parent temporary folder which will be holding input
 *              data used by test-case
 *
 * @t: Reference to testutil
 * @folderName: Parent folder name
 *
 * Output: On success return path
 *         On Failure exit program
 */
func makeTempDir(t *testing.T, folderName string) (string) {
	tmpDir, err := os.MkdirTemp("", folderName)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	return tmpDir
}

/*
 * Test: TestLustre2LnetMetrics
 * Purpose: Verify /sys/kernel/debug/lnet/stats
 * TestFolder: testcases/TestLustre2LnetMetrics
 * InputFile: lnetProcContents.out
 * Example:
 *   # cat /sys/kernel/debug/lnet/stats
 *   0 6 0 1420 1420 0 0 1004048 1004048 0 0
 */
func TestLustre2LnetMetrics(t *testing.T) {
	/* Read testcases/TestLustre2LnetMetrics folder */
	data := ReadDir(t, "testcases", "TestLustre2LnetMetrics",
			"lnetProcContents.out")

	 /* create stats file under temp folder */
	tmpDir := makeTempDir(t, "telegraf-lustre")
	tempdir := tmpDir + "/telegraf/sys/kernel/debug/"
	lnetdir := tempdir + "/lnet"
	err := os.MkdirAll(lnetdir, 0750)
	require.NoError(t, err)

	/* write values read from testcases/lnet/expected.out under stats */
	err = os.WriteFile(lnetdir + "/stats", []byte(data), 0640)
	require.NoError(t, err)

	m := &Lustre2{
		LnetProcfiles: []string{lnetdir + "/stats"},
	}

	var acc testutil.Accumulator
	/* Get Actual Value to verify. Call Gather() method to read actual
	 * data. In this case it will read from temporary file created above
	 */
	err = m.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"name":   "lnet",
	}

	/* Expected Value */
	fields := map[string]interface{}{
		"lnet_msgs_alloc":      uint64(0),
		"lnet_msgs_max":        uint64(7),
		"lnet_rst_alloc":       uint64(0),
		"lnet_send_count":      uint64(20481),
		"lnet_recv_count":      uint64(28239),
		"lnet_route_count":     uint64(0),
		"lnet_drop_count":      uint64(0),
		"lnet_send_length":     uint64(8892268623),
		"lnet_recv_length":     uint64(8225856),
		"lnet_route_length":    uint64(0),
		"lnet_drop_length":     uint64(0),
	}
	/* Verify for fields against "lustre2" measurement */
	acc.AssertContainsTaggedFields(t, "lustre2", fields, tags)
}

/*
 * Test: TestLustre2GeneratesJobstatsMetrics
 * Purpose: Verify /proc/fs/lustre/mdt/lustre-MDT0000/job_stats
 *          Verify /proc/fs/lustre/obdfilter/lustre-OST0000/job_stats
 * TestFolder: testcases/TestLustre2GeneratesJobstatsMetrics
 * InputFile: mdtJobStatsContents.out
 * InputFile: obdfilterJobStatsContents.out
 */
func TestLustre2GeneratesJobstatsMetrics(t *testing.T) {
	/* Read testcases/TestLustre2GeneratesJobstatsMetrics folder */
	data := ReadDir(t, "testcases",
			"TestLustre2GeneratesJobstatsMetrics",
			"mdtJobStatsContents.out")
	data1 := ReadDir(t, "testcases",
			"TestLustre2GeneratesJobstatsMetrics",
			"obdfilterJobStatsContents.out")

	tmpDir := makeTempDir(t, "telegraf-lustre-jobstats")
	tempdir := tmpDir + "/telegraf/proc/fs/lustre/"
	ostName := "OST0001"
	jobNames := []string{"cluster-testjob1", "testjob2"}

	mdtdir := tempdir + "/mdt/"
	err := os.MkdirAll(mdtdir+"/"+ostName, 0750)
	require.NoError(t, err)

	obddir := tempdir + "/obdfilter/"
	err = os.MkdirAll(obddir+"/"+ostName, 0750)
	require.NoError(t, err)

	err = os.WriteFile(mdtdir+"/"+ostName+"/job_stats", []byte(data), 0640)
	require.NoError(t, err)

	err = os.WriteFile(obddir+"/"+ostName+"/job_stats", []byte(data1), 0640)
	require.NoError(t, err)

	// Test Lustre Jobstats
	m := &Lustre2{
		OstProcfiles: []string{obddir + "/*/job_stats"},
		MdsProcfiles: []string{mdtdir + "/*/job_stats"},
	}

	var acc testutil.Accumulator

	err = m.Gather(&acc)
	require.NoError(t, err)

	// make this two tags
	// and even further make this dependent on summing per OST
	tags := []map[string]string{
		{
			"name":  ostName,
			"jobid": jobNames[0],
		},
		{
			"name":  ostName,
			"jobid": jobNames[1],
		},
	}

	// make this for two tags
	fields := []map[string]interface{}{
		{
			"jobstats_read_calls":      uint64(1),
			"jobstats_read_min_size":   uint64(4096),
			"jobstats_read_max_size":   uint64(4096),
			"jobstats_read_bytes":      uint64(4096),
			"jobstats_write_calls":     uint64(25),
			"jobstats_write_min_size":  uint64(1048576),
			"jobstats_write_max_size":  uint64(16777216),
			"jobstats_write_bytes":     uint64(26214400),
			"jobstats_ost_getattr":     uint64(0),
			"jobstats_ost_setattr":     uint64(0),
			"jobstats_punch":           uint64(1),
			"jobstats_ost_sync":        uint64(0),
			"jobstats_destroy":         uint64(0),
			"jobstats_create":          uint64(0),
			"jobstats_ost_statfs":      uint64(0),
			"jobstats_get_info":        uint64(0),
			"jobstats_set_info":        uint64(0),
			"jobstats_quotactl":        uint64(0),
			"jobstats_open":            uint64(5),
			"jobstats_close":           uint64(4),
			"jobstats_mknod":           uint64(6),
			"jobstats_link":            uint64(8),
			"jobstats_unlink":          uint64(90),
			"jobstats_mkdir":           uint64(521),
			"jobstats_rmdir":           uint64(520),
			"jobstats_rename":          uint64(9),
			"jobstats_getattr":         uint64(11),
			"jobstats_setattr":         uint64(1),
			"jobstats_getxattr":        uint64(3),
			"jobstats_setxattr":        uint64(4),
			"jobstats_statfs":          uint64(1205),
			"jobstats_sync":            uint64(2),
			"jobstats_samedir_rename":  uint64(705),
			"jobstats_crossdir_rename": uint64(200),
		},
		{
			"jobstats_read_calls":      uint64(1),
			"jobstats_read_min_size":   uint64(1024),
			"jobstats_read_max_size":   uint64(1024),
			"jobstats_read_bytes":      uint64(1024),
			"jobstats_write_calls":     uint64(25),
			"jobstats_write_min_size":  uint64(2048),
			"jobstats_write_max_size":  uint64(2048),
			"jobstats_write_bytes":     uint64(51200),
			"jobstats_ost_getattr":     uint64(0),
			"jobstats_ost_setattr":     uint64(0),
			"jobstats_punch":           uint64(1),
			"jobstats_ost_sync":        uint64(0),
			"jobstats_destroy":         uint64(0),
			"jobstats_create":          uint64(0),
			"jobstats_ost_statfs":      uint64(0),
			"jobstats_get_info":        uint64(0),
			"jobstats_set_info":        uint64(0),
			"jobstats_quotactl":        uint64(0),
			"jobstats_open":            uint64(6),
			"jobstats_close":           uint64(7),
			"jobstats_mknod":           uint64(8),
			"jobstats_link":            uint64(9),
			"jobstats_unlink":          uint64(20),
			"jobstats_mkdir":           uint64(200),
			"jobstats_rmdir":           uint64(210),
			"jobstats_rename":          uint64(8),
			"jobstats_getattr":         uint64(10),
			"jobstats_setattr":         uint64(2),
			"jobstats_getxattr":        uint64(4),
			"jobstats_setxattr":        uint64(5),
			"jobstats_statfs":          uint64(1207),
			"jobstats_sync":            uint64(3),
			"jobstats_samedir_rename":  uint64(706),
			"jobstats_crossdir_rename": uint64(201),
		},
	}

	for index := 0; index < len(fields); index++ {
		acc.AssertContainsTaggedFields(t, "lustre2", fields[index],
					       tags[index])
	}
}

/*
 * Test: TestLustre2GeneratesClientMetrics
 * Purpose: Verify /proc/fs/lustre/mdt/lustre-MDT0000/exports/0\@lo/stats
 *          Verify /proc/fs/lustre/obdfilter/lustre-OST0000/exports/0\@lo/stats
 * TestFolder: testcases/TestLustre2GeneratesClientMetrics
 * InputFile: mdtProcContents.out
 * InputFile: obdfilterProcContents.out
 */
func TestLustre2GeneratesClientMetrics(t *testing.T) {
	data := ReadDir(t, "testcases",
			"TestLustre2GeneratesClientMetrics",
			"mdtProcContents.out")
	data1 := ReadDir(t, "testcases",
			"TestLustre2GeneratesClientMetrics",
			"obdfilterProcContents.out")
	tmpDir := makeTempDir(t, "telegraf-lustre-client")

	tempdir := tmpDir + "/telegraf/proc/fs/lustre/"
	ostName := "OST0001"
	clientName := "10.2.4.27@o2ib1"
	mdtdir := tempdir + "/mdt/"
	err := os.MkdirAll(mdtdir+"/"+ostName+"/exports/"+clientName, 0750)
	require.NoError(t, err)

	obddir := tempdir + "/obdfilter/"
	err = os.MkdirAll(obddir+"/"+ostName+"/exports/"+clientName, 0750)
	require.NoError(t, err)

	err = os.WriteFile(mdtdir+"/"+ostName+"/exports/"+clientName+"/stats",
			   []byte(data), 0640)
	require.NoError(t, err)

	err = os.WriteFile(obddir+"/"+ostName+"/exports/"+clientName+"/stats",
			   []byte(data1), 0640)
	require.NoError(t, err)

	// Begin by testing standard Lustre stats
	m := &Lustre2{
		OstProcfiles: []string{obddir + "/*/exports/*/stats"},
		MdsProcfiles: []string{mdtdir + "/*/exports/*/stats"},
	}

	var acc testutil.Accumulator

	err = m.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"name":   ostName,
		"client": clientName,
	}

	fields := map[string]interface{}{
		"close":           uint64(873243496),
		"crossdir_rename": uint64(369571),
		"getattr":         uint64(1503663097),
		"getxattr":        uint64(6145349681),
		"link":            uint64(445),
		"mkdir":           uint64(705499),
		"mknod":           uint64(349042),
		"open":            uint64(1024577037),
		"read_bytes":      uint64(78026117632000),
		"read_calls":      uint64(203238095),
		"rename":          uint64(629196),
		"rmdir":           uint64(227434),
		"samedir_rename":  uint64(259625),
		"setattr":         uint64(1898364),
		"setxattr":        uint64(83969),
		"statfs":          uint64(2916320),
		"sync":            uint64(434081),
		"unlink":          uint64(3549417),
		"write_bytes":     uint64(15201500833981),
		"write_calls":     uint64(71893382),
	}

	acc.AssertContainsTaggedFields(t, "lustre2", fields, tags)
}

/*
 * Test: TestLustre2GeneratesMetrics
 * Purpose: Verify /proc/fs/lustre/mdt/lustre-MDT0000/md_stats
 *          Verify /proc/fs/lustre/obdfilter/lustre-OST0000/stats
 *          Verify /sys/kernel/debug/lustre/osd-ldiskfs/lustre-OST0000/stats
 * TestFolder: testcases/TestLustre2GeneratesMetrics
 * InputFile: mdtProcContents.out
 * InputFile: osdldiskfsProcContents.out
 * InputFile: obdfilterProcContents.out
 */
func TestLustre2GeneratesMetrics(t *testing.T) {
	data := ReadDir(t, "testcases",
			"TestLustre2GeneratesMetrics",
			"mdtProcContents.out")
	data1 := ReadDir(t, "testcases",
			"TestLustre2GeneratesMetrics",
			"osdldiskfsProcContents.out")
	data2 := ReadDir(t, "testcases",
			"TestLustre2GeneratesMetrics",
			"obdfilterProcContents.out")
	tmpDir := makeTempDir(t, "telegraf-lustre")

	tempdir := tmpDir + "/telegraf/proc/fs/lustre/"
	ostName := "OST0001"

	mdtdir := tempdir + "/mdt/"
	err := os.MkdirAll(mdtdir+"/"+ostName, 0750)
	require.NoError(t, err)

	osddir := tempdir + "/osd-ldiskfs/"
	err = os.MkdirAll(osddir+"/"+ostName, 0750)
	require.NoError(t, err)

	obddir := tempdir + "/obdfilter/"
	err = os.MkdirAll(obddir+"/"+ostName, 0750)
	require.NoError(t, err)

	err = os.WriteFile(mdtdir+"/"+ostName+"/md_stats",
			   []byte(data), 0640)
	require.NoError(t, err)

	err = os.WriteFile(osddir+"/"+ostName+"/stats",
			   []byte(data1), 0640)
	require.NoError(t, err)

	err = os.WriteFile(obddir+"/"+ostName+"/stats",
			   []byte(data2), 0640)
	require.NoError(t, err)

	// Begin by testing standard Lustre stats
	m := &Lustre2{
		OstProcfiles: []string{obddir + "/*/stats", osddir + "/*/stats"},
		MdsProcfiles: []string{mdtdir + "/*/md_stats"},
	}
	var acc testutil.Accumulator

	err = m.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"name": ostName,
	}

	fields := map[string]interface{}{
		"cache_access":    uint64(19047063027),
		"cache_hit":       uint64(7393729777),
		"cache_miss":      uint64(11653333250),
		"close":           uint64(873243496),
		"crossdir_rename": uint64(369571),
		"getattr":         uint64(1503663097),
		"getxattr":        uint64(6145349681),
		"link":            uint64(445),
		"mkdir":           uint64(705499),
		"mknod":           uint64(349042),
		"open":            uint64(1024577037),
		"read_bytes":      uint64(78026117632000),
		"read_calls":      uint64(203238095),
		"rename":          uint64(629196),
		"rmdir":           uint64(227434),
		"samedir_rename":  uint64(259625),
		"setattr":         uint64(1898364),
		"setxattr":        uint64(83969),
		"statfs":          uint64(2916320),
		"sync":            uint64(434081),
		"unlink":          uint64(3549417),
		"write_bytes":     uint64(15201500833981),
		"write_calls":     uint64(71893382),
	}

	acc.AssertContainsTaggedFields(t, "lustre2", fields, tags)
}

/*
 * Test: TestLustre2CanParseConfiguration
 * Purpose: Verify Config file can be read correctly
 * TestFolder: testcases/TestLustre2CanParseConfiguration
 * InputFile: config.out
 */
func TestLustre2CanParseConfiguration(t *testing.T) {
	data := ReadDir(t, "testcases",
			"TestLustre2CanParseConfiguration",
			"config.out")
	table, err := toml.Parse(data)
	require.NoError(t, err)

	inputs, ok := table.Fields["inputs"]
	require.True(t, ok)

	lustre2, ok := inputs.(*ast.Table).Fields["lustre2"]
	require.True(t, ok)

	var plugin Lustre2

	require.NoError(t, toml.UnmarshalTable(lustre2.([]*ast.Table)[0],
					       &plugin))

	require.Equal(t, Lustre2{
		OstProcfiles: []string{
			"/proc/fs/lustre/obdfilter/*/stats",
			"/proc/fs/lustre/osd-ldiskfs/*/stats",
		},
		MdsProcfiles: []string{
			"/proc/fs/lustre/mdt/*/md_stats",
		},
		LnetProcfiles: []string{
			"/sys/kernel/debug/lnet/stats",
		},
	}, plugin)
}

